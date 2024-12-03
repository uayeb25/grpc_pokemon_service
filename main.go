package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

func (s *server) GetPokemonList(req *pb.Empty, stream pb.PokemonService_GetPokemonListServer) error {
	query := "select * from pokeprd.Pokemon"
	rows, err := db.Query(query)

	if err != nil {
		log.Panic(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, ptype string
		var level int

		if err := rows.Scan(&name, &ptype, &level); err != nil {
			log.Panic(err)
			return err
		}

		if err := stream.Send(&pb.PokemonResponse{
			Name:  name,
			Type:  ptype,
			Level: int32(level),
		}); err != nil {
			log.Panic(err)
			return err
		}
	}

	return nil

}

func (s *server) AddPokemons(stream pb.PokemonService_AddPokemonsServer) error {
	var count int32

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.AddPokemonResponse{
				Count: count,
			})
		}

		if err != nil {
			log.Panic(err)
			return err
		}

		query := "insert into pokeprd.Pokemon (Name, Type, Level) values (@Name, @Type, @Level)"
		_, err = db.Exec(query,
			sql.Named("Name", req.Name),
			sql.Named("Type", req.Type),
			sql.Named("Level", req.Level))

		if err != nil {
			log.Panic(err)
			return err
		}

		count++
		log.Printf("Added %s", req.Name)

	}

}

func (s *server) GetPokemonsByType(stream pb.PokemonService_GetPokemonsByTypeServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			log.Println("End of stream")
			return nil
		}

		if err != nil {
			log.Panic(err)
			return err
		}

		query := "select * from pokeprd.Pokemon where lower(Type) = lower(@Type) "
		rows, err := db.Query(query, sql.Named("Type", req.Type))
		if err != nil {
			log.Panic(err)
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var name, ptype string
			var level int

			if err := rows.Scan(&name, &ptype, &level); err != nil {
				log.Panic(err)
				return err
			}

			if err := stream.Send(&pb.PokemonResponse{
				Name:  name,
				Type:  ptype,
				Level: int32(level),
			}); err != nil {
				log.Panic(err)
				return err
			}
		}

	}
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

	// Iniciar servidor HTTP para health check
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		log.Println("Starting health check server on port 8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	log.Println("Starting server on port :50051")
	if err := grpcServer.Serve(listener); err != nil {
		panic(err)
	}

}
