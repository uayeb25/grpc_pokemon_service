package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	pb "pokemon-grpc/proto"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

var db *sql.DB

type server struct {
	pb.UnimplementedPokemonServiceServer
}

func (s *server) GetPokemonInfo(ctx context.Context, req *pb.PokemonRequest) (*pb.PokemonResponse, error) {
	var name, ptype string
	var level int

	query := "select * from pokeprd.Pokemon where Name like @Name"

	row := db.QueryRowContext(ctx, query, sql.Named("Name", "%"+req.Name+"%"))
	err := row.Scan(&name, &ptype, &level)

	if err != nil {
		if err == sql.ErrNoRows {
			return &pb.PokemonResponse{
				Name:  "Not Found",
				Type:  "Not Found",
				Level: 0,
			}, nil
		}
		return nil, err
	}

	return &pb.PokemonResponse{
		Name:  name,
		Type:  ptype,
		Level: int32(level),
	}, nil

}

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	s := os.Getenv("DB_SERVER")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	database := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	connString := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
		user, password, s, port, database)

	db, err = sql.Open("sqlserver", connString)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(err)
	}
	log.Println("Connected to database")

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPokemonServiceServer(grpcServer, &server{})

	log.Println("Starting server on port :50051")
	if err := grpcServer.Serve(listener); err != nil {
		panic(err)
	}

}
