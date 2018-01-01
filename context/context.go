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

	Associate(user string, cluster string) bool
	UsersByCluster() map[string][]string // cluster -> []user
	ClustersByUser() map[string][]string // user -> []cluster
	Dissociate(user string, cluster string) bool

	Commit() error
}
