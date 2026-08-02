package conf

type Terminal struct {
	NpmPackageManager  string                        `mapstructure:"npm_package_manager" json:"npm_package_manager" yaml:"npm_package_manager"`
	InstallServicePort string                        `mapstructure:"install_service_port" json:"install_service_port" yaml:"install_service_port"`
	Commands           map[string]map[string]Command `mapstructure:"commands" json:"commands" yaml:"commands"`
}

type Command struct {
	Cwd     string `mapstructure:"cwd" json:"cwd" yaml:"cwd"`
	Command string `mapstructure:"Command" json:"Command" yaml:"Command"`
}
