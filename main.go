package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/luminalteam/lmdrouter"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type AppConfig struct {
	LogLevel string
}

type AppParams struct {
	fx.In

	Config *AppConfig
	Logger *zap.Logger
}

func NewConfig() (*AppConfig, error) {
	viper.SetDefault("log_level", "info")

	viper.AutomaticEnv()

	var config AppConfig
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func NewLogger(config *AppConfig) (*zap.Logger, error) {
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(config.LogLevel)); err != nil {
		return nil, err
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		os.Stdout,
		level,
	)

	logger := zap.New(core)

	return logger, nil
}

func NewRouter() *lmdrouter.Router {
	router := lmdrouter.NewRouter()

	router.GET("/", func(ctx context.Context, req *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
		return &events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       "Hello, World!",
		}, nil
	})

	return router
}

func HandleRequest(params AppParams, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger := params.Logger.With(zap.String("correlation_id", req.RequestContext.RequestID))

	logger.Info("Handling request")

	router := NewRouter()

	response, err := router.Handle(context.Background(), &req)

	if err != nil {
		logger.Error("Error handling request", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	logger.Info("Request handled successfully")

	return *response, nil
}

func main() {
	app := fx.New(
		fx.Provide(
			NewConfig,
			NewLogger,
			NewRouter,
		),
		fx.Invoke(
			func(logger *zap.Logger) {
				defer logger.Sync()
			},
		),
	)

	lambda.StartHandler(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		var response events.APIGatewayProxyResponse

		err := app.Start(context.Background())
		if err != nil {
			log.Fatalf("Error starting application: %v", err)
		}

		defer func() {
			err := app.Stop(context.Background())
			if err != nil {
				log.Fatalf("Error stopping application: %v", err)
			}
		}()

		err = app.Invoke(func(params AppParams) {
			response, err = HandleRequest(params, req)
		})
		if err != nil {
			log.Fatalf("Error invoking application: %v", err)
		}

		return response, err
	})
}
