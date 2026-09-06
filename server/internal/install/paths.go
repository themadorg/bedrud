package install

// Standard paths used by install, update, and uninstall.
const (
	etcConfigPath     = "/etc/bedrud/config.yaml"
	etcLivekitPath    = "/etc/bedrud/livekit.yaml"
	etcDir            = "/etc/bedrud"
	varLibDir         = "/var/lib/bedrud"
	varLogDir         = "/var/log/bedrud"
	versionFilePath   = "/var/lib/bedrud/version"
	binaryLocalPath    = "/usr/local/bin/bedrud"
	binaryPackagePath  = "/usr/bin/bedrud"
	defaultInstallRoot = "/opt/bedrud"
	envInstallRoot     = "BEDRUD_INSTALL_ROOT"
	envNoVMService     = "BEDRUD_VERSION_MANAGER_NO_SERVICE"
	// Default FHS doc paths (runtime vars in examples_embed.go allow test override).
	defaultDocDir         = "/usr/share/doc/bedrud"
	defaultDocExamplesDir = "/usr/share/doc/bedrud/examples"
)
