package context

type Context interface {
	SetUser(value string)
	User() string
	UserPrevious() string
	Users() []string
	SetCluster(value string)
	Cluster() string
	ClusterPrevious() string
	Clusters() []string
	SetNamespace(value string)
	Namespace() string
	NamespacePrevious() string
	Namespaces() ([]string, error)
	NamespaceView() ([]string, error)

	Associate(user string, cluster string) bool
	UsersByCluster() map[string][]string // cluster -> []user
	ClustersByUser() map[string][]string // user -> []cluster
	Dissociate(user string, cluster string) bool

	ExplicitNamespaces() []FQNS
	SetExplicitNamespace(user string, cluster string, namespace string) bool
	DeleteExplicitNamespace(user string, cluster string, namespace string) bool

	Commit() error
}

type FQNS struct {
	User    string
	Cluster string
	NS      string
}
