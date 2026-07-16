package snowflake

type SnowFlaker interface {
	GenerateID() int64
}
