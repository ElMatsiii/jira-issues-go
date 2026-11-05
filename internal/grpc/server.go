package grpc

import (
	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/kafka"
	"jira-issues-go/internal/repository"
	"jira-issues-go/internal/service"
)

// Server implements issuesv1.IssueServiceServer.
type Server struct {
	issuesv1.UnimplementedIssueServiceServer
	svc service.IssueService
}

// NewServer constructs a Server from repository and event producer.
func NewServer(repo repository.IssueRepository, producer kafka.EventProducer) *Server {
	svc := service.NewIssueService(repo, producer)
	return &Server{
		svc: svc,
	}
}

// NewIssueHandler constructs a Server directly from an IssueService.
func NewIssueHandler(svc service.IssueService) *Server {
	return &Server{
		svc: svc,
	}
}
