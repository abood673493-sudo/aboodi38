module ssh-manager-panel

go 1.22

require (
	github.com/go-chi/chi/v5 v5.0.12
	github.com/go-chi/cors v1.2.1
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/gorilla/websocket v1.5.1
	github.com/jackc/pgx/v5 v5.5.5
	github.com/joho/godotenv v1.5.1
	golang.org/x/crypto v0.21.0
	golang.org/x/term v0.18.0
	golang.org/x/time v0.5.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace golang.org/x/crypto => github.com/golang/crypto v0.21.0

replace golang.org/x/term => github.com/golang/term v0.18.0

replace golang.org/x/time => github.com/golang/time v0.5.0

replace golang.org/x/net => github.com/golang/net v0.21.0

replace golang.org/x/text => github.com/golang/text v0.14.0

replace golang.org/x/sys => github.com/golang/sys v0.18.0

replace golang.org/x/sync => github.com/golang/sync v0.1.0

replace gopkg.in/yaml.v3 => ./third_party/yamlstub
