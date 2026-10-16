module github.com/jellyfish-p/ephyra/server

go 1.25.0

require github.com/jellyfish-p/ephyra/protocol v0.0.0

require (
	github.com/google/uuid v1.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/jellyfish-p/ephyra/protocol => ../protocol
