package core

import (
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"go.uber.org/zap"
)

type Core struct {
	Store  filestore.FileStore
	Logger *zap.Logger
}

func New(store filestore.FileStore, logger *zap.Logger) *Core {
	return &Core{Store: store, Logger: logger}
}
