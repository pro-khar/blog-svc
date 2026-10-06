FROM golang:1.26
RUN go install -tags='no_azuresql no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' github.com/pressly/goose/v3/cmd/goose@v3.27.0
ENTRYPOINT ["goose"]