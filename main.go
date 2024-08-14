package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zinclabs/otel-example/models"
	"github.com/zinclabs/otel-example/pkg/tel"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	tp := tel.InitTracerHTTP()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			fmt.Println("Error shutting down tracer provider: ", err)
		}
	}()

	router := gin.Default()

	router.Use(otelgin.Middleware(""))

	router.GET("/user", GetUser)
	router.POST("/user", PostUser)

	router.Run(":8080")

}

func GetUser(c *gin.Context) {
	span := trace.SpanFromContext(c.Request.Context())
	ctx := trace.ContextWithSpan(c.Request.Context(), span)

	defer span.End()

	// error.type - the error with which the operation ended
	// http.response.status_code - since we have a server setup, this is to be setup only in case of error.

	// Set custom HTTP semantic attributes
	span.SetAttributes(
		attribute.String("http.request.method", c.Request.Method),
		attribute.String("url.path", c.Request.URL.String()),
		attribute.String("http.query", c.Request.URL.RawQuery),
		attribute.String("http.scheme", c.Request.URL.Scheme),
	)

	details, err := GetUserDetails(ctx)
	if err != nil {
		span.SetAttributes(
			attribute.String("error.type", err.Error()),
			attribute.Int("http.response.status_code", http.StatusInternalServerError))

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching user details)"})
	}

	// If successful, return the user info
	c.JSON(http.StatusOK, gin.H{
		"user": details,
	})
}

func GetUserDetails(ctx context.Context) ([]models.User, error) {
	var (
		user []models.User
		cur  *mongo.Cursor
	)

	client, err := createCon(ctx)
	if err != nil {
		return user, err
	}

	coll := client.Database("db").Collection(models.UsersCol)
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

func PostUser(c *gin.Context) {
	span := trace.SpanFromContext(c.Request.Context())
	ctx := trace.ContextWithSpan(c.Request.Context(), span)

	defer span.End()

	// Set custom HTTP semantic attributes
	span.SetAttributes(
		attribute.String("http.request.method", c.Request.Method),
		attribute.String("url.path", c.Request.URL.String()),
		attribute.String("http.query", c.Request.URL.RawQuery),
		attribute.String("http.scheme", c.Request.URL.Scheme),
	)
	user := models.User{}
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	details, err := PostUserDetails(ctx, user)
	if err != nil {
		log.Println("Error posting user details: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error posting user details"})
	}

	// If successful, return the user info
	c.JSON(http.StatusOK, gin.H{
		"user": details,
	})
}

func PostUserDetails(ctx context.Context, user models.User) (models.User, error) {
	client, err := createCon(ctx)
	if err != nil {
		log.Println("Error connecting to MongoDB: ", err)
		return user, err
	}

	coll := client.Database("db").Collection(models.UsersCol)
	_, err = coll.InsertOne(ctx, &user)
	if err != nil {
		log.Println("Error inserting in MongoDB: ", err)
		return user, err
	}

	return user, err
}

func createCon(ctx context.Context) (client *mongo.Client, err error) {
	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://root:example@localhost:27017"))
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)

	return client, err
}
