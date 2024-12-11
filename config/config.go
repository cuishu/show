package config

type Config struct {
	Key   string `gorm:"column:key"`
	Value string `gorm:"column:Value"`
}
