package boards

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dboard "github.com/tandem/tandem/internal/http/dto/board"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/board/core"
	"github.com/tandem/tandem/internal/usecase/board/mapping"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
)

var defaultColumns = mcolumn.DefaultColumnNames

type Boards struct {
	*core.Core
}

func New(c *core.Core) *Boards {
	return &Boards{Core: c}
}

func (s *Boards) Create(ctx context.Context, actorID, workspaceID string, req dboard.CreateBoardRequest) (*dboard.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireEditor(member.Role); err != nil {
		return nil, err
	}
	if err := validate.Name(req.Name); err != nil {
		return nil, err
	}
	existing, err := s.Boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	board := &mboard.Board{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		Name:        strings.TrimSpace(req.Name),
		Position:    len(existing),
	}
	if err := s.Boards.CreateBoard(ctx, board); err != nil {
		return nil, err
	}
	for i, name := range defaultColumns {
		column := &mcolumn.Column{
			ID:       uuid.New().String(),
			BoardID:  board.ID,
			Name:     name,
			Position: i,
		}
		if err := s.Columns.CreateColumn(ctx, column); err != nil {
			return nil, err
		}
	}
	response := mapping.BoardToResponse(board)
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: core.EventBoardCreated, Data: response})
	return response, nil
}

func (s *Boards) List(ctx context.Context, actorID, workspaceID string) ([]dboard.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	_, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.Cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+actorID)
	listKey := fmt.Sprintf("u:%s:t:v1:boards:%s:%s:%s", actorID, workspaceID, wsver, uver)
	var cached []dboard.BoardResponse
	if cacheutil.Load(ctx, s.Cache, listKey, &cached) {
		return cached, nil
	}
	boards, err := s.Boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	counts, err := s.Boards.CountTasksByBoard(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	responses := make([]dboard.BoardResponse, 0, len(boards))
	favBoards, err := s.Favorites.ListFavoriteTargets(ctx, actorID, mfavorite.FavoriteBoard)
	if err != nil {
		return nil, err
	}
	for i := range boards {
		resp := mapping.BoardToResponse(boards[i])
		resp.TaskCount = counts[boards[i].ID]
		resp.IsFavorite = favBoards[boards[i].ID]
		responses = append(responses, *resp)
	}
	cacheutil.Store(ctx, s.Cache, listKey, responses, cacheutil.TTL)
	return responses, nil
}

func (s *Boards) Get(ctx context.Context, actorID, workspaceID, boardID string) (*dboard.BoardDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	board, err := s.BoardInWorkspace(ctx, workspaceID, boardID)
	if err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.Cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+actorID)
	detailKey := fmt.Sprintf("u:%s:t:v1:board:%s:%s:%s", actorID, boardID, wsver, uver)
	var cached dboard.BoardDetailResponse
	if cacheutil.Load(ctx, s.Cache, detailKey, &cached) {
		return &cached, nil
	}
	columns, err := s.Columns.ListColumns(ctx, board.ID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.Tasks.ListTasksForBoard(ctx, board.ID)
	if err != nil {
		return nil, err
	}
	workspace, err := s.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	users, err := s.ResolveUsers(ctx, tasks)
	if err != nil {
		return nil, err
	}
	columnDetails := mapping.BuildColumnDetails(columns, tasks, users, workspace.Prefix, boardID, board.Name)
	detail := &dboard.BoardDetailResponse{
		ID:          board.ID,
		WorkspaceID: board.WorkspaceID,
		Name:        board.Name,
		Position:    board.Position,
		IsMain:      board.IsMain,
		Columns:     columnDetails,
		CreatedAt:   board.CreatedAt,
		UpdatedAt:   board.UpdatedAt,
	}
	cacheutil.Store(ctx, s.Cache, detailKey, detail, cacheutil.TTL)
	return detail, nil
}

func (s *Boards) Update(ctx context.Context, actorID, workspaceID, boardID string, req dboard.UpdateBoardRequest) (*dboard.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireEditor(member.Role); err != nil {
		return nil, err
	}
	if err := validate.Name(req.Name); err != nil {
		return nil, err
	}
	board, err := s.BoardInWorkspace(ctx, workspaceID, boardID)
	if err != nil {
		return nil, err
	}
	board.Name = strings.TrimSpace(req.Name)
	if err := s.Boards.UpdateBoard(ctx, board); err != nil {
		return nil, err
	}
	response := mapping.BoardToResponse(board)
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: core.EventBoardUpdated, Data: response})
	return response, nil
}

func (s *Boards) Delete(ctx context.Context, actorID, workspaceID, boardID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(boardID); err != nil {
		return err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.RequireEditor(member.Role); err != nil {
		return err
	}
	board, err := s.BoardInWorkspace(ctx, workspaceID, boardID)
	if err != nil {
		return err
	}
	if board.IsMain {
		return pkgerrors.NewValidationError("cannot delete the main board")
	}
	keys, err := s.Tasks.CollectBoardKeys(ctx, board.ID)
	if err != nil {
		return err
	}
	s.Files.RemoveMany(ctx, keys)
	if err := s.Boards.DeleteBoard(ctx, board.ID); err != nil {
		return err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{
		Type: core.EventBoardDeleted,
		Data: map[string]string{"id": board.ID},
	})
	return nil
}

func (s *Boards) SetMain(ctx context.Context, actorID, workspaceID, boardID string) (*dboard.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireOwner(member.Role); err != nil {
		return nil, err
	}
	board, err := s.BoardInWorkspace(ctx, workspaceID, boardID)
	if err != nil {
		return nil, err
	}
	if err := s.Boards.ClearMainBoards(ctx, workspaceID); err != nil {
		return nil, err
	}
	board.IsMain = true
	if err := s.Boards.UpdateBoard(ctx, board); err != nil {
		return nil, err
	}
	response := mapping.BoardToResponse(board)
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: core.EventBoardUpdated, Data: response})
	return response, nil
}

func (s *Boards) Reorder(ctx context.Context, actorID, workspaceID string, req dboard.ReorderBoardsRequest) ([]dboard.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireEditor(member.Role); err != nil {
		return nil, err
	}
	boards, err := s.Boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := validateReorderIDs(req.BoardIDs, boards); err != nil {
		return nil, err
	}
	if err := s.Boards.ReorderBoards(ctx, workspaceID, req.BoardIDs); err != nil {
		return nil, err
	}
	counts, err := s.Boards.CountTasksByBoard(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	ordered := orderBoards(req.BoardIDs, boards, counts)
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: core.EventBoardsReordered, Data: ordered})
	return ordered, nil
}

func validateReorderIDs(boardIDs []string, boards []*mboard.Board) error {
	if len(boardIDs) == 0 {
		return pkgerrors.NewValidationError("board_ids is required")
	}
	valid := make(map[string]bool, len(boards))
	for _, b := range boards {
		valid[b.ID] = true
	}
	seen := make(map[string]bool, len(boardIDs))
	for _, id := range boardIDs {
		if !valid[id] {
			return pkgerrors.NewValidationError("board_ids contains a board not in this workspace")
		}
		if seen[id] {
			return pkgerrors.NewValidationError("board_ids contains duplicates")
		}
		seen[id] = true
	}
	return nil
}

func orderBoards(boardIDs []string, boards []*mboard.Board, counts map[string]int) []dboard.BoardResponse {
	ordered := make([]dboard.BoardResponse, 0, len(boardIDs))
	for _, id := range boardIDs {
		for _, b := range boards {
			if b.ID == id {
				resp := mapping.BoardToResponse(b)
				resp.Position = len(ordered)
				resp.TaskCount = counts[id]
				ordered = append(ordered, *resp)
				break
			}
		}
	}
	return ordered
}
