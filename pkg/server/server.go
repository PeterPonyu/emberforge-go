package server

import "fmt"

type Config struct {
	Port int
}

type Server struct {
	config Config
}

func New(config Config) Server {
	return Server{config: config}
}

func (s Server) Describe() string {
	return fmt.Sprintf("Server listening on port %d", s.config.Port)
}
