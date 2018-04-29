package kubectl

import (
	log "github.com/Sirupsen/logrus"
	nsx "github.com/shyiko/kubensx/context"
	"k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	k8sclientcmd "k8s.io/client-go/tools/clientcmd"
	k8sclientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sort"
	"strings"
)

const (
	assocPrefix    = "kubensx-assoc:"
	assocSeparator = ":"
	nsPrefix       = "kubensx-ns:"
	nsSeparator    = "/"
	contextCurrent = "kubensx-current"
	contextPrev    = "kubensx-prev"
)

type context struct {
	pre                   *k8sclientcmdapi.Context
	acs                   k8sclientcmd.ConfigAccess
	cfg                   *k8sclientcmdapi.Config
	nss                   func(user string, cluster string) ([]string, error)
	currentContextMutated bool
}

type contextRef struct {
	key string
	ctx *k8sclientcmdapi.Context
}

func (ctx *context) SetUser(value string) {
	ctx.mutateCurrentNSX(func(ctx *k8sclientcmdapi.Context) {
		ctx.AuthInfo = value
	})
}

func (ctx *context) User() string {
	return currentNSX(ctx).AuthInfo
}

func (ctx *context) UserPrevious() string {
	return previousNSX(ctx).AuthInfo
}

func (ctx *context) Users() []string {
	keys := make([]string, 0, len(ctx.cfg.AuthInfos))
	for key := range ctx.cfg.AuthInfos {
		keys = append(keys, key)
	}
	return keys
}

func (ctx *context) SetCluster(value string) {
	ctx.mutateCurrentNSX(func(ctx *k8sclientcmdapi.Context) {
		ctx.Cluster = value
	})
}

func (ctx *context) Cluster() string {
	return currentNSX(ctx).Cluster
}

func (ctx *context) ClusterPrevious() string {
	return previousNSX(ctx).Cluster
}

func (ctx *context) Clusters() []string {
	keys := make([]string, 0, len(ctx.cfg.Clusters))
	for key := range ctx.cfg.Clusters {
		keys = append(keys, key)
	}
	return keys
}

func (ctx *context) SetNamespace(value string) {
	ctx.mutateCurrentNSX(func(ctx *k8sclientcmdapi.Context) {
		ctx.Namespace = value
	})
}

func (ctx *context) Namespace() string {
	return currentNSX(ctx).Namespace
}

func (ctx *context) NamespacePrevious() string {
	return previousNSX(ctx).Namespace
}

func (ctx *context) Namespaces() ([]string, error) {
	r, err := ctx.nss(ctx.User(), ctx.Cluster())
	if statusError, ok := err.(*errors.StatusError); ok && statusError.ErrStatus.Code == 403 {
		return r, nil
	}
	return r, err
}

func (ctx *context) NamespaceView() ([]string, error) {
	var r []string
	user := ctx.User()
	cluster := ctx.Cluster()
	for _, ref := range ctx.ExplicitNamespaces() {
		if ref.User == user && ref.Cluster == cluster {
			r = append(r, ref.NS)
		}
	}
	if len(r) > 0 {
		return r, nil
	}
	return ctx.Namespaces()
}

func (ctx *context) Associate(user string, cluster string) bool {
	key := assocKey(user, cluster)
	if ctx.cfg.Contexts[key] != nil {
		return false
	}
	ctx.cfg.Contexts[key] = &k8sclientcmdapi.Context{AuthInfo: user, Cluster: cluster}
	return true
}

func assocKey(user string, cluster string) string {
	return assocPrefix + user + assocSeparator + cluster
}

func (ctx *context) UsersByCluster() map[string][]string {
	m := make(map[string][]string)
	ctx.forEachAssoc(func(user string, cluster string) {
		m[cluster] = append(m[cluster], user)
	})
	return m
}

func (ctx *context) ClustersByUser() map[string][]string {
	m := make(map[string][]string)
	ctx.forEachAssoc(func(user string, cluster string) {
		m[user] = append(m[user], cluster)
	})
	return m
}

func (ctx *context) forEachAssoc(cb func(string, string)) {
	for key := range ctx.cfg.Contexts {
		if strings.HasPrefix(key, assocPrefix) {
			pair := strings.TrimPrefix(key, assocPrefix)
			idx := strings.LastIndex(pair, assocSeparator)
			if idx != -1 {
				user, cluster := pair[:idx], pair[idx+1:]
				if ctx.cfg.AuthInfos[user] != nil && ctx.cfg.Clusters[cluster] != nil {
					cb(user, cluster)
				}
			}
		}
	}
}

func (ctx *context) Dissociate(user string, cluster string) bool {
	key := assocKey(user, cluster)
	if ctx.cfg.Contexts[key] == nil {
		return false
	}
	delete(ctx.cfg.Contexts, key)
	return true
}

func (ctx *context) ExplicitNamespaces() []nsx.FQNS {
	var r []nsx.FQNS
	for key := range ctx.cfg.Contexts {
		if strings.HasPrefix(key, nsPrefix) {
			pair := strings.TrimPrefix(key, nsPrefix)
			idx := strings.LastIndex(pair, nsSeparator)
			if idx != -1 {
				split := strings.SplitN(pair[:idx], assocSeparator, 2)
				if len(split) == 2 {
					r = append(r, nsx.FQNS{User: split[0], Cluster: split[1], NS: pair[idx+1:]})
				}
			}
		}
	}
	return r
}

func (ctx *context) SetExplicitNamespace(user string, cluster string, namespace string) bool {
	key := nsKey(user, cluster, namespace)
	if ctx.cfg.Contexts[key] != nil {
		return false
	}
	ctx.cfg.Contexts[key] = &k8sclientcmdapi.Context{AuthInfo: user, Cluster: cluster, Namespace: namespace}
	return true
}

func (ctx *context) DeleteExplicitNamespace(user string, cluster string, namespace string) bool {
	key := nsKey(user, cluster, namespace)
	if ctx.cfg.Contexts[key] == nil {
		return false
	}
	delete(ctx.cfg.Contexts, key)
	return true
}

func nsKey(user string, cluster string, namespace string) string {
	return nsPrefix + user + assocSeparator + cluster + nsSeparator + namespace
}

func (ctx *context) Commit() error {
	if ctx.currentContextMutated {
		if ctx.pre != nil {
			ctx.cfg.Contexts[contextPrev] = ctx.pre
			log.Debugf(`Set "%s" to "%s:%s/%s"`, contextPrev, ctx.pre.AuthInfo, ctx.pre.Cluster, ctx.pre.Namespace)
		}
		curr := ctx.cfg.Contexts[ctx.cfg.CurrentContext]
		log.Debugf(`Set "%s" to "%s:%s/%s"`, contextCurrent, curr.AuthInfo, curr.Cluster, curr.Namespace)
	}
	ctx.purgeInvalid()
	k8sclientcmd.ModifyConfig(ctx.acs, *ctx.cfg, false)
	return nil
}

func (ctx *context) purgeInvalid() {
	var keys []string
	for key := range ctx.cfg.Contexts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if strings.HasPrefix(key, assocPrefix) {
			pair := strings.TrimPrefix(key, assocPrefix)
			path := strings.SplitN(pair, assocSeparator, 2)
			if len(path) == 2 && ctx.cfg.AuthInfos[path[0]] != nil && ctx.cfg.Clusters[path[1]] != nil {
				log.Debugf(`Found assoc[iation] "%s"`, key)
				continue
			}
			log.Debugf(`Deleted assoc[iation] "%s"`, key)
			delete(ctx.cfg.Contexts, key)
		} else if strings.HasPrefix(key, nsPrefix) {
			triple := strings.TrimPrefix(key, nsPrefix)
			idx := strings.LastIndex(triple, nsSeparator)
			if idx != -1 {
				path := strings.SplitN(triple[:idx], assocSeparator, 2)
				if len(path) == 2 && ctx.cfg.AuthInfos[path[0]] != nil && ctx.cfg.Clusters[path[1]] != nil {
					log.Debugf(`Found explicit ns "%s"`, key)
					continue
				}
			}
			log.Debugf(`Deleted explicit ns "%s"`, key)
			delete(ctx.cfg.Contexts, key)
		}
	}
}

func (ctx *context) mutateCurrentNSX(cb func(ctx *k8sclientcmdapi.Context)) {
	ctx.currentContextMutated = true
	ref := currentNSXRef(ctx)
	if ref.key == contextCurrent {
		cb(ref.ctx)
		return
	}
	k8sctx := &k8sclientcmdapi.Context{
		AuthInfo:  ref.ctx.AuthInfo,
		Cluster:   ref.ctx.Cluster,
		Namespace: ref.ctx.Namespace,
	}
	ctx.cfg.Contexts[contextCurrent] = k8sctx
	ctx.cfg.CurrentContext = contextCurrent
	cb(k8sctx)
}

func currentNSX(ctx *context) *k8sclientcmdapi.Context {
	return currentNSXRef(ctx).ctx
}

func currentNSXRef(ctx *context) *contextRef {
	currentContext := ctx.cfg.Contexts[ctx.cfg.CurrentContext]
	if currentContext == nil {
		currentContext = ctx.cfg.Contexts[contextCurrent]
		if currentContext == nil {
			currentContext = &k8sclientcmdapi.Context{}
			ctx.cfg.Contexts[contextCurrent] = currentContext
		}
		ctx.cfg.CurrentContext = contextCurrent
	}
	return &contextRef{ctx.cfg.CurrentContext, currentContext}
}

func previousNSX(ctx *context) *k8sclientcmdapi.Context {
	r := ctx.cfg.Contexts[contextPrev]
	if r == nil {
		r = currentNSX(ctx)
	}
	return r
}

func newContext(nss func(cfg k8sclientcmdapi.Config) func(user string, cluster string) ([]string, error)) (nsx.Context, error) {
	clientConfig := k8sclientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		k8sclientcmd.NewDefaultClientConfigLoadingRules(),
		&k8sclientcmd.ConfigOverrides{},
	)
	cfg, err := clientConfig.RawConfig()
	if err != nil {
		return nil, err
	}
	ctx := cfg.Contexts[cfg.CurrentContext]
	if ctx != nil {
		ctx = ctx.DeepCopy()
	}
	return &context{pre: ctx, acs: clientConfig.ConfigAccess(), cfg: &cfg, nss: nss(cfg)}, nil
}

func NewContext() (nsx.Context, error) {
	// this method will have to be rewritten if ctx.Namespaces() is ever executed more than once over the course
	// of single command execution
	return newContext(func(cfg k8sclientcmdapi.Config) func(user string, cluster string) ([]string, error) {
		return func(user string, cluster string) ([]string, error) {
			def := k8sclientcmd.NewDefaultClientConfigLoadingRules()
			override := &k8sclientcmd.ConfigOverrides{
				Context: k8sclientcmdapi.Context{AuthInfo: user, Cluster: cluster},
			}
			log.Debugf(`Initializing client with "%s:%s"`, override.Context.AuthInfo, override.Context.Cluster)
			clientConfig, err := k8sclientcmd.NewNonInteractiveDeferredLoadingClientConfig(def, override).ClientConfig()
			if err != nil {
				return nil, err
			}
			client, err := k8s.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			nss, err := client.CoreV1().Namespaces().List(k8smetav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			acc := make([]string, 0, len(nss.Items))
			for _, ns := range nss.Items {
				acc = append(acc, ns.Name)
			}
			return acc, nil
		}
	})
}

func NewContextStub(nss func(user string, cluster string) ([]string, error)) (nsx.Context, error) {
	return newContext(func(cfg k8sclientcmdapi.Config) func(user string, cluster string) ([]string, error) { return nss })
}
