migrate:
	cd sql/schema && goose postgres "postgres://postgres:postgres@localhost:5432/gator" up

queries:
	sqlc generate
