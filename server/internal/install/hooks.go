package install

// System-operation hooks used by install/update. Tests replace these with no-ops
// so LinuxUpdate can run without root or real systemd.

var (
	createUserHook      = createBedrudUser
	chownRHook          = runChownR
	chownHook           = runChown
	stopServicesHook    = stopAllInitSystems
	refreshServicesHook = refreshServices
	packageManagedHook  = isPackageManaged
	confirmUpdateHook   = confirmGitHubUpdate
)
