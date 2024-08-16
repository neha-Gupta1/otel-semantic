package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zinclabs/otel-example/pkg/tel"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var UsersCol = "users"

type Users struct {
	ID      string `json:"id" binding:"required"`
	Name    string `json:"name" binding:"required"`
	PhoneNo int    `json:"phone_no" binding:"required"`
}

// A mock function to simulate user authentication
func authenticate(c *gin.Context) (string, error) {
	token := c.GetHeader("Authorization")
	if token == "" || !strings.HasPrefix(token, "Bearer ") {
		return "", errors.New("missing or invalid token")
	}
	// In a real-world application, you would validate the token here
	// For simplicity, we'll just extract the token and pretend it's the username
	return strings.TrimPrefix(token, "Bearer "), nil
}

// Middleware for authentication
func authMiddleware(c *gin.Context, span trace.Span) error {
	username, err := authenticate(c)
	if err != nil {
		// Add an event to the span, indicating an error
		span.AddEvent("Error fetching user details", trace.WithAttributes(
			attribute.String("event.category", err.Error()),
			attribute.String("event.type", "auth"),
			attribute.String("error.message", err.Error()),
			attribute.String("user.name", username),
		))
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return err
	}

	// Attach username to the context
	c.Set("username", username)
	c.Next()
	return nil
}

func main() {
	// Initialize tracing
	tp := tel.InitTracerHTTP()
	defer tp.Shutdown(context.Background())

	router := gin.Default()

	// OpenTelemetry Gin middleware
	router.Use(otelgin.Middleware("user-service"))

	router.GET("/user", GetUser)
	router.POST("/user", PostUser)

	router.Run(":8080")
}

func GetUser(c *gin.Context) {
	ctx, span := trace.SpanFromContext(c.Request.Context()).TracerProvider().Tracer("").Start(c.Request.Context(), "GetUser")
	defer span.End()

	username := c.GetString("username")
	span.SetAttributes(attribute.String("user.name", username))

	authMiddleware(c, span)

	details, err := GetUserDetails(ctx, span)
	if err != nil {
		// Add an event to the span, indicating an error
		span.AddEvent("Error fetching user details", trace.WithAttributes(
			attribute.String("event.category", "error"),
			attribute.String("event.type", "db"),
			attribute.String("error.message", err.Error()),
			attribute.String("user.name", username),
		))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching user details)"})
		return
	}

	span.AddEvent("User details retrieved", trace.WithAttributes(
		attribute.String("event.category", "database"),
		attribute.String("event.type", "query"),
		attribute.String("db.system", "mongodb"),
		attribute.String("http.method", "GET"),
		attribute.String("user.name", username),
	))

	// If successful, return the user info
	c.JSON(http.StatusOK, gin.H{
		"user": details,
	})
}

func PostUser(c *gin.Context) {
	ctx, span := trace.SpanFromContext(c.Request.Context()).TracerProvider().Tracer("").Start(c.Request.Context(), "PostUser")
	defer span.End()

	username := c.GetString("username")
	span.SetAttributes(attribute.String("user.name", username))

	err := authMiddleware(c, span)
	if err != nil {
		return
	}

	user := Users{}
	if err := c.ShouldBindJSON(&user); err != nil {
		// Add an event to the span for input validation failure
		span.AddEvent("Validation Error", trace.WithAttributes(
			attribute.String("event.category", "validation"),
			attribute.String("event.type", "error"),
			attribute.String("http.method", "POST"),
			attribute.String("error.message", err.Error()),
			attribute.String("user.name", username),
		))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	details, err := PostUserDetails(ctx, span, user)
	if err != nil {
		// Add an event to the span indicating a database error
		span.AddEvent("Error posting user details", trace.WithAttributes(
			attribute.String("event.category", "error"),
			attribute.String("event.type", "db"),
			attribute.String("db.system", "mongodb"),
			attribute.String("error.message", err.Error()),
			attribute.String("user.name", username),
		))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error posting user details"})
		return
	}

	// Add a successful event for the user creation
	span.AddEvent("User details posted", trace.WithAttributes(
		attribute.String("event.category", "database"),
		attribute.String("event.type", "insert"),
		attribute.String("db.system", "mongodb"),
		attribute.String("http.method", "POST"),
		attribute.String("user.name", username),
	))

	// If successful, return the user info
	c.JSON(http.StatusOK, gin.H{
		"user": details,
	})
}

func GetUserDetails(ctx context.Context, span trace.Span) ([]Users, error) {
	var (
		user []Users
		cur  *mongo.Cursor
	)

	client, err := createCon(ctx, span)
	if err != nil {
		return user, err
	}

	span.SetAttributes(
		attribute.String("db.collection.name", UsersCol),
		attribute.String("db.namespace", "db"),
		attribute.String("db.query.text", "{}"),
		attribute.String("db.operation.name", "findAll"),
	)

	coll := client.Database("db").Collection(UsersCol)
	cur, err = coll.Find(ctx, bson.M{})
	if err != nil {
		fmt.Println("Error connecting to MongoDB: ", err)
		return user, err
	}

	defer func() {
		cur.Close(ctx)
	}()

	err = cur.All(ctx, &user)
	if err != nil {
		log.Println("Error getting user details: ", err)
		return user, err
	}

	return user, nil
}

func PostUserDetails(ctx context.Context, span trace.Span, user Users) (Users, error) {
	client, err := createCon(ctx, span)
	if err != nil {
		log.Println("Error connecting to MongoDB: ", err)
		return user, err
	}

	span.SetAttributes(
		attribute.String("db.collection.name", UsersCol),
		attribute.String("db.namespace", "db"),
		attribute.String("db.operation.name", "InsertOne"),
	)

	coll := client.Database("db").Collection(UsersCol)
	_, err = coll.InsertOne(ctx, &user)
	if err != nil {
		log.Println("Error inserting in MongoDB: ", err)
		return user, err
	}

	return user, err
}

func createCon(ctx context.Context, span trace.Span) (client *mongo.Client, err error) {
	// error.type
	serverAddress := "localhost"
	serverPort := "27017"
	database := "mongodb"

	span.SetAttributes(
		attribute.String("db.system", database),
		attribute.String("server.address", serverAddress),
		attribute.String("server.port", serverPort),
	)

	client, err = mongo.Connect(ctx, options.Client().ApplyURI(fmt.Sprintf("%s://root:example@%s:%s", database, serverAddress, serverPort)))
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)

	return client, err
}
