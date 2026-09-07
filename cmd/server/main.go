package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"task-management/backend/internal/db"
	"task-management/backend/internal/email"
	"task-management/backend/internal/handler"
	"task-management/backend/internal/middleware"
	"task-management/backend/internal/storage"
)

func main() {
	_ = godotenv.Load()

	database, err := db.New()
	if err != nil {
		log.Fatalf("DB接続に失敗しました: %v", err)
	}

	router := gin.Default()
	router.HandleMethodNotAllowed = false

	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "https://akifumi1119.github.io")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "API is running"})
	})

	emailClient := email.NewClient(
		os.Getenv("RESEND_API_KEY"),
		os.Getenv("RESEND_FROM"),
	)

	var storageClient *storage.Client
	if cloudURL := os.Getenv("CLOUDINARY_URL"); cloudURL != "" {
		var err error
		storageClient, err = storage.NewClient(cloudURL)
		if err != nil {
			log.Printf("Cloudinary初期化に失敗しました（画像アップロード無効）: %v", err)
		}
	}

	authHandler := handler.NewAuthHandler(database, emailClient)
	projectHandler := handler.NewProjectHandler(database)
	taskHandler := handler.NewTaskHandler(database, storageClient)
	commentHandler := handler.NewCommentHandler(database, storageClient)
	userHandler := handler.NewUserHandler(database)

	api := router.Group("/api")
	api.POST("/login", authHandler.Login)
	api.POST("/users", authHandler.Register)
	api.GET("/users/check", authHandler.CheckUser)
	api.POST("/verify-email", authHandler.VerifyEmail)
	api.POST("/resend-verification", authHandler.ResendVerification)

	authorized := api.Group("", middleware.JWTAuth())
	authorized.POST("/logout", authHandler.Logout)
	authorized.GET("/users/:id", userHandler.GetUser)
	authorized.PUT("/users/:id", userHandler.UpdateUser)
	authorized.DELETE("/users/:id", userHandler.DeleteUser)
	authorized.PUT("/users/:id/password", userHandler.ChangePassword)
	authorized.GET("/projects/:id", projectHandler.GetProjects)
	authorized.POST("/projects", projectHandler.CreateProject)
	authorized.GET("/projects/:id/members", projectHandler.GetProjectMembers)
	authorized.GET("/projects/:id/authority", projectHandler.GetAuthority)
	authorized.DELETE("/projects/:id", projectHandler.DeleteProject)
	authorized.PUT("/projects", projectHandler.UpdateProject)
	authorized.GET("/tasks", taskHandler.GetTasks)
	authorized.GET("/tasks/:task_id", taskHandler.GetTask)
	authorized.PUT("/tasks/:task_id", taskHandler.UpdateTask)
	authorized.POST("/tasks", taskHandler.CreateTask)
	authorized.DELETE("/tasks/:task_id", taskHandler.DeleteTask)
	authorized.POST("/comments/:task_id", commentHandler.CreateComment)
	authorized.PUT("/comments/:comment_id", commentHandler.UpdateComment)
	authorized.DELETE("/comments/:comment_id", commentHandler.DeleteComment)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}
