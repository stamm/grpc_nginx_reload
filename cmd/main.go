//go:generate protoc -I ../helloworld --go_out=plugins=grpc:../helloworld ../helloworld/helloworld.proto

package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	pb "github.com/stamm/grpc_nginx_reload/helloworld"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	port = ":50082"
)

// server is used to implement helloworld.GreeterServer.
type server struct{}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func (s *server) SayHelloStream(stream pb.Greeter_SayHelloStreamServer) error {
	var names []string
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			repl := &pb.HelloReply{Message: "Hello " + strings.Join(names, ", ")}
			return stream.SendAndClose(repl)
		}
		if err != nil {
			return err
		}
		names = append(names, in.Name)
	}
}

func (s *server) SayHelloStream2(in *pb.HelloRequest, stream pb.Greeter_SayHelloStream2Server) error {
	count := in.Count
	if count <= 0 {
		count = 1
	}
	for i := int64(0); i < count; i++ {
		err := stream.Send(&pb.HelloReply{Message: fmt.Sprintf("Hello%d %s", i, in.Name)})
		if err != nil {
			return err
		}
	}
	time.Sleep(time.Duration(in.Sleep) * time.Second)
	return nil
}

func (s *server) SayHelloStream3(stream pb.Greeter_SayHelloStream3Server) error {
	sleep := int64(0)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			time.Sleep(time.Duration(sleep) * time.Second)
			return nil
		}
		if err != nil {
			return err
		}
		sleep = in.Sleep
		count := in.Count
		if count <= 0 {
			count = 1
		}
		for i := int64(0); i < count; i++ {
			err := stream.Send(&pb.HelloReply{Message: fmt.Sprintf("Hello%d %s", i, in.Name)})
			if err != nil {
				return err
			}
		}
	}
}

// EchoServer the data received on the WebSocket.
func EchoServer(ws *websocket.Conn) {
	// time.Sleep(30 * time.Second)
	io.Copy(ws, ws)
}

func websocketServer() {
	http.Handle("/echo", websocket.Handler(EchoServer))
	err := http.ListenAndServe(":50052", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

func httpServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, longpoll")
	})
	http.HandleFunc("/sleep", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(60 * time.Second)
		fmt.Fprintf(w, "Hello, longpoll")
	})

	log.Fatal(http.ListenAndServe(":50080", nil))
}

func main() {
	go httpServer()
	go websocketServer()
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	log.Printf("Started")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
