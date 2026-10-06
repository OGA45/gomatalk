module github.com/OGA45/gomatalk

go 1.25.3

require (
	github.com/boltdb/bolt v1.3.1
	github.com/bwmarrin/dgvoice v0.0.0-00010101000000-000000000000
	github.com/bwmarrin/discordgo v0.29.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/golang-migrate/migrate/v4 v4.19.0
	github.com/spf13/viper v1.21.0
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.0
)

require (
	github.com/Carmen-Shannon/gopus v1.0.0 // indirect
	github.com/disgoorg/godave v0.1.0 // indirect
	github.com/disgoorg/godave/golibdave v0.1.0 // indirect
	github.com/disgoorg/godave/libdave v0.1.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.32 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)

replace github.com/bwmarrin/discordgo => ./discordgo-dave

replace github.com/bwmarrin/dgvoice => ./dgvoice
