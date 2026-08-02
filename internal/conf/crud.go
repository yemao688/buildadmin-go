package conf

type Crud struct {
	ApplyOnMigrate bool `mapstructure:"apply_on_migrate" json:"apply_on_migrate" yaml:"apply_on_migrate"`
}
