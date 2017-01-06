package ocicni

const (
	DefaultInterfaceName = "eth0"
	CNIPluginName        = "cni"
	DefaultNetDir        = "/etc/cni/net.d"
	DefaultCNIDir        = "/opt/cni/bin"
	VendorCNIDirTemplate = "%s/opt/%s/bin"
)

type CNIPlugin interface {
	// Name returns the plugin's name. This will be used when searching
	// for a plugin by name, e.g.
	Name() string

	// SetUpPod is the method called after the infra container of
	// the pod has been created but before the other containers of the
	// pod are launched.
	SetUpPod(netnsPath string, namespace string, name string, containerID string) error

	// TearDownPod is the method called before a pod's infra container will be deleted
	TearDownPod(netnsPath string, namespace string, name string, containerID string) error

	// NetworkStatus returns error if the network plugin is in error state
	Status() error
}
