package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"task-management/backend/internal/db"
	"task-management/backend/internal/handler"
	"task-management/backend/internal/middleware"
)

func main() {
	database, err := db.New()
	if err != nil {
		log.Fatalf("DB接続に失敗しました: %v", err)
	}

	router := gin.Default()

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

	authHandler := handler.NewAuthHandler(database)
	projectHandler := handler.NewProjectHandler(database)
	taskHandler := handler.NewTaskHandler(database)
	commentHandler := handler.NewCommentHandler(database)

	api := router.Group("/api")
	api.POST("/login", authHandler.Login)
	api.POST("/users", authHandler.Register)
	api.GET("/users/check", authHandler.CheckUser)

	authorized := api.Group("", middleware.JWTAuth())
	authorized.POST("/logout", authHandler.Logout)
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
