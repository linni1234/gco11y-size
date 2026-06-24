package main

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	pb "github.com/acme/shop-go/gen"
	shopconnect "github.com/acme/shop-go/gen/shopconnect"
	g "github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/mux"
	"github.com/labstack/echo/v4"
	"github.com/nats-io/nats.go"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
)

type greeterServer struct{}

func main() {
	http.HandleFunc("GET /health", health)
	http.HandleFunc("/legacy", legacy)

	r := g.Default()
	api := r.Group("/api")
	api.GET("/orders/:id", getOrder)
	api.POST("/orders", createOrder)
	RegisterGinRoutes(api.Group("/registered"))

	e := echo.New()
	v1 := e.Group("/v1")
	v1.PUT("/customers/:id", updateCustomer)

	cr := chi.NewRouter()
	cr.Get("/chi/{id}", chiHandler)
	cr.Route("/nested", func(r chi.Router) {
		r.Post("/{id}", chiHandler)
	})

	gm := mux.NewRouter()
	gm.HandleFunc("/gorilla/{id}", gorillaHandler).Methods("GET")

	app := fiber.New()
	app.Get("/fiber/:id", fiberHandler)

	_, _ = http.Get("https://payments.internal/charge")
	_, _ = http.NewRequest("POST", "https://inventory.svc.cluster.local/items", nil)
	_, _ = grpc.Dial("inventory-service:50051")

	_ = kafka.NewReader(kafka.ReaderConfig{Topic: "orders.created"})
	writer := kafka.NewWriter(kafka.WriterConfig{Topic: "audit.events"})
	_ = writer.WriteMessages(context.Background(), kafka.Message{Topic: "audit.events"})

	var ch *amqp.Channel
	_, _ = ch.Consume("orders.queue", "", true, false, false, false, nil)
	_ = ch.Publish("events", "order.created", false, false, amqp.Publishing{})

	var nc *nats.Conn
	_, _ = nc.Subscribe("inventory.updated", func(msg *nats.Msg) {})
	_ = nc.Publish("billing.created", nil)

	tracer := otel.Tracer("shop")
	_, span := tracer.Start(context.Background(), "custom-go-work")
	span.SetAttributes(attribute.String("user.id", "123"))

	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &greeterServer{})
	_, _ = shopconnect.NewGreeterHandler(&greeterServer{})
	_ = connect.NewError
}

func (greeterServer) SayHello(context.Context, *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{}, nil
}

func RegisterGinRoutes(router *g.RouterGroup) {
	router.GET("/helper/:id", getRegistered)
}

func health(http.ResponseWriter, *http.Request)         {}
func legacy(http.ResponseWriter, *http.Request)         {}
func getOrder(*g.Context)                               {}
func createOrder(*g.Context)                            {}
func getRegistered(*g.Context)                          {}
func updateCustomer(echo.Context) error                 { return nil }
func chiHandler(http.ResponseWriter, *http.Request)     {}
func gorillaHandler(http.ResponseWriter, *http.Request) {}
func fiberHandler(*fiber.Ctx) error                     { return nil }
