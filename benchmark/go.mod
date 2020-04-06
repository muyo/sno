module github.com/muyo/sno/benchmark

go 1.14

require (
	github.com/bwmarrin/snowflake v0.3.0
	github.com/celrenheit/sandflake v0.0.0-20190410195419-50a943690bc2
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/lucsky/cuid v1.0.2
	github.com/muyo/sno v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/rs/xid v1.2.1
	github.com/segmentio/ksuid v1.0.2
	github.com/sony/sonyflake v1.0.0
)

replace github.com/muyo/sno => ../
