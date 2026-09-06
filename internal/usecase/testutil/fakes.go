package testutil

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"gorm.io/gorm"
)

type FakeWorkspaceRepo struct {
	Workspaces map[string]*models.Workspace
	Members    map[string]map[string]*models.WorkspaceMember
}

func NewFakeWorkspaceRepo() *FakeWorkspaceRepo {
	return &FakeWorkspaceRepo{
		Workspaces: make(map[string]*models.Workspace),
		Members:    make(map[string]map[string]*models.WorkspaceMember),
	}
}

func (f *FakeWorkspaceRepo) AddMemberFixture(wsID, userID string, role models.WorkspaceRole) {
	if f.Members[wsID] == nil {
		f.Members[wsID] = make(map[string]*models.WorkspaceMember)
	}
	f.Members[wsID][userID] = &models.WorkspaceMember{WorkspaceID: wsID, UserID: userID, Role: role, CreatedAt: time.Now()}
}

func (f *FakeWorkspaceRepo) AddWorkspaceFixture(id, name string) {
	f.Workspaces[id] = &models.Workspace{ID: id, Name: name, Prefix: "WS", CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func (f *FakeWorkspaceRepo) CreateWorkspace(_ context.Context, ws *models.Workspace) error {
	f.Workspaces[ws.ID] = &models.Workspace{
		ID: ws.ID, Name: ws.Name, Description: ws.Description, Prefix: ws.Prefix, Theme: ws.Theme,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return nil
}

func (f *FakeWorkspaceRepo) FindWorkspaceByID(_ context.Context, id string) (*models.Workspace, error) {
	ws, ok := f.Workspaces[id]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return ws, nil
}

func (f *FakeWorkspaceRepo) FindWorkspaceByInvite(_ context.Context, token string) (*models.Workspace, error) {
	for _, ws := range f.Workspaces {
		if ws.InviteToken != "" && ws.InviteToken == token {
			return ws, nil
		}
	}
	return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
}

func (f *FakeWorkspaceRepo) UpdateWorkspace(_ context.Context, ws *models.Workspace) error {
	existing, ok := f.Workspaces[ws.ID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	existing.Name = ws.Name
	existing.Description = ws.Description
	existing.Prefix = ws.Prefix
	existing.Theme = ws.Theme
	existing.InviteToken = ws.InviteToken
	return nil
}

func (f *FakeWorkspaceRepo) UpdateInviteToken(_ context.Context, workspaceID, token string) error {
	ws, ok := f.Workspaces[workspaceID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	ws.InviteToken = token
	return nil
}

func (f *FakeWorkspaceRepo) DeleteWorkspace(_ context.Context, id string) error {
	if _, ok := f.Workspaces[id]; !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	delete(f.Workspaces, id)
	delete(f.Members, id)
	return nil
}

func (f *FakeWorkspaceRepo) ListWorkspacesForUser(_ context.Context, userID string) ([]models.WorkspaceMembership, error) {
	var out []models.WorkspaceMembership
	for wsID, mm := range f.Members {
		if m, ok := mm[userID]; ok {
			out = append(out, models.WorkspaceMembership{
				Workspace: *f.Workspaces[wsID],
				Role:      m.Role,
			})
		}
	}
	return out, nil
}

func (f *FakeWorkspaceRepo) AddMember(_ context.Context, wsID, userID string, role models.WorkspaceRole) error {
	if f.Members[wsID] == nil {
		f.Members[wsID] = make(map[string]*models.WorkspaceMember)
	}
	if _, ok := f.Members[wsID][userID]; ok {
		return pkgerrors.Wrap(pkgerrors.ErrConflict, errors.New("unique constraint"))
	}
	f.Members[wsID][userID] = &models.WorkspaceMember{WorkspaceID: wsID, UserID: userID, Role: role, CreatedAt: time.Now()}
	return nil
}

func (f *FakeWorkspaceRepo) FindMember(_ context.Context, wsID, userID string) (*models.WorkspaceMember, error) {
	m, ok := f.Members[wsID][userID]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return m, nil
}

func (f *FakeWorkspaceRepo) RemoveMember(_ context.Context, wsID, userID string) error {
	if _, ok := f.Members[wsID][userID]; !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	delete(f.Members[wsID], userID)
	return nil
}

func (f *FakeWorkspaceRepo) UpdateMemberRole(_ context.Context, wsID, userID string, role models.WorkspaceRole) error {
	m, ok := f.Members[wsID][userID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	m.Role = role
	return nil
}

func (f *FakeWorkspaceRepo) ListMembers(_ context.Context, wsID string) ([]models.WorkspaceMember, error) {
	var out []models.WorkspaceMember
	for _, m := range f.Members[wsID] {
		out = append(out, *m)
	}
	return out, nil
}

func (f *FakeWorkspaceRepo) DeleteMembersByWorkspace(_ context.Context, wsID string) error {
	delete(f.Members, wsID)
	return nil
}

var _ repository.WorkspaceRepository = (*FakeWorkspaceRepo)(nil)

type FakeUserRepo struct {
	Users map[string]*models.User
}

func NewFakeUserRepo() *FakeUserRepo {
	return &FakeUserRepo{Users: make(map[string]*models.User)}
}

func (f *FakeUserRepo) AddUserFixture(id, login string) *models.User {
	user := &models.User{ID: id, Login: login, DisplayName: login, Role: models.RoleUser, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.Users[id] = user
	return user
}

func (f *FakeUserRepo) Create(_ context.Context, user *models.User) error {
	f.Users[user.ID] = user
	return nil
}

func (f *FakeUserRepo) FindByID(_ context.Context, id string) (*models.User, error) {
	user, ok := f.Users[id]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return user, nil
}

func (f *FakeUserRepo) FindByLogin(_ context.Context, login string) (*models.User, error) {
	for _, user := range f.Users {
		if user.Login == login {
			return user, nil
		}
	}
	return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
}

func (f *FakeUserRepo) ExistsByLogin(_ context.Context, login string) (bool, error) {
	_, err := f.FindByLogin(context.Background(), login)
	if err != nil {
		if errors.Is(err, pkgerrors.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f *FakeUserRepo) List(_ context.Context) ([]*models.User, error) {
	var out []*models.User
	for _, user := range f.Users {
		out = append(out, user)
	}
	return out, nil
}

func (f *FakeUserRepo) ListPage(_ context.Context, _ string, _ int, _ int) ([]*models.User, error) {
	return f.List(context.Background())
}

func (f *FakeUserRepo) Count(_ context.Context, _ string) (int, error) {
	return len(f.Users), nil
}

func (f *FakeUserRepo) Update(_ context.Context, user *models.User) error {
	f.Users[user.ID] = user
	return nil
}

func (f *FakeUserRepo) Delete(_ context.Context, id string) error {
	delete(f.Users, id)
	return nil
}

var _ repository.UserRepository = (*FakeUserRepo)(nil)

type FakeBoardRepo struct {
	Boards map[string]*models.Board
}

func NewFakeBoardRepo() *FakeBoardRepo {
	return &FakeBoardRepo{Boards: make(map[string]*models.Board)}
}

func (f *FakeBoardRepo) AddBoardFixture(id, workspaceID, name string) *models.Board {
	board := &models.Board{ID: id, WorkspaceID: workspaceID, Name: name, Position: len(f.Boards), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.Boards[id] = board
	return board
}

func (f *FakeBoardRepo) CreateBoard(_ context.Context, board *models.Board) error {
	board.CreatedAt = time.Now()
	board.UpdatedAt = time.Now()
	f.Boards[board.ID] = board
	return nil
}

func (f *FakeBoardRepo) FindBoardByID(_ context.Context, id string) (*models.Board, error) {
	board, ok := f.Boards[id]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return board, nil
}

func (f *FakeBoardRepo) UpdateBoard(_ context.Context, board *models.Board) error {
	existing, ok := f.Boards[board.ID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	existing.Name = board.Name
	existing.Position = board.Position
	existing.IsMain = board.IsMain
	existing.ArchivedAt = board.ArchivedAt
	return nil
}

func (f *FakeBoardRepo) ClearMainBoards(_ context.Context, workspaceID string) error {
	for _, board := range f.Boards {
		if board.WorkspaceID == workspaceID {
			board.IsMain = false
		}
	}
	return nil
}

func (f *FakeBoardRepo) DeleteBoard(_ context.Context, id string) error {
	if _, ok := f.Boards[id]; !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	delete(f.Boards, id)
	return nil
}

func sortBoardsByPosition(boards []*models.Board) {
	for i := 1; i < len(boards); i++ {
		for j := i; j > 0 && boards[j-1].Position > boards[j].Position; j-- {
			boards[j-1], boards[j] = boards[j], boards[j-1]
		}
	}
}

func (f *FakeBoardRepo) ListBoards(_ context.Context, workspaceID string) ([]*models.Board, error) {
	var out []*models.Board
	for _, board := range f.Boards {
		if board.WorkspaceID == workspaceID {
			out = append(out, board)
		}
	}
	sortBoardsByPosition(out)
	return out, nil
}

func (f *FakeBoardRepo) CountTasksByBoard(_ context.Context, _ string) (map[string]int, error) {
	return map[string]int{}, nil
}

func (f *FakeBoardRepo) ReorderBoards(_ context.Context, workspaceID string, boardIDs []string) error {
	for i, id := range boardIDs {
		if b, ok := f.Boards[id]; ok && b.WorkspaceID == workspaceID {
			b.Position = i
		}
	}
	return nil
}

var _ repository.BoardRepository = (*FakeBoardRepo)(nil)

type FakeColumnRepo struct {
	Columns         map[string]*models.Column
	BoardWorkspaces map[string]string
}

func NewFakeColumnRepo() *FakeColumnRepo {
	return &FakeColumnRepo{Columns: make(map[string]*models.Column), BoardWorkspaces: make(map[string]string)}
}

func (f *FakeColumnRepo) RegisterBoard(workspaceID, boardID string) {
	f.BoardWorkspaces[boardID] = workspaceID
}

func (f *FakeColumnRepo) AddColumnFixture(id, boardID, name string, position int) *models.Column {
	column := &models.Column{ID: id, BoardID: boardID, Name: name, Position: position, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.Columns[id] = column
	return column
}

func (f *FakeColumnRepo) CreateColumn(_ context.Context, column *models.Column) error {
	column.CreatedAt = time.Now()
	column.UpdatedAt = time.Now()
	f.Columns[column.ID] = column
	return nil
}

func (f *FakeColumnRepo) FindColumnByID(_ context.Context, id string) (*models.Column, error) {
	column, ok := f.Columns[id]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return column, nil
}

func sortColumnsByPosition(columns []*models.Column) {
	for i := 1; i < len(columns); i++ {
		for j := i; j > 0 && columns[j-1].Position > columns[j].Position; j-- {
			columns[j-1], columns[j] = columns[j], columns[j-1]
		}
	}
}

func (f *FakeColumnRepo) ListColumns(_ context.Context, boardID string) ([]*models.Column, error) {
	var out []*models.Column
	for _, column := range f.Columns {
		if column.BoardID == boardID {
			out = append(out, column)
		}
	}
	sortColumnsByPosition(out)
	return out, nil
}

func (f *FakeColumnRepo) ListColumnsForWorkspace(_ context.Context, workspaceID string) ([]*models.Column, error) {
	var out []*models.Column
	for _, column := range f.Columns {
		if f.BoardWorkspaces[column.BoardID] == workspaceID {
			out = append(out, column)
		}
	}
	sortColumnsByPosition(out)
	return out, nil
}

var _ repository.ColumnRepository = (*FakeColumnRepo)(nil)

type FakeTaskRepo struct {
	Tasks            map[string]*models.Task
	ColumnBoards     map[string]string
	ColumnWorkspaces map[string]string
}

func NewFakeTaskRepo() *FakeTaskRepo {
	return &FakeTaskRepo{Tasks: make(map[string]*models.Task), ColumnBoards: make(map[string]string), ColumnWorkspaces: make(map[string]string)}
}

func (f *FakeTaskRepo) RegisterColumn(columnID, boardID string) {
	f.ColumnBoards[columnID] = boardID
}

func (f *FakeTaskRepo) RegisterColumnWorkspace(columnID, workspaceID string) {
	f.ColumnWorkspaces[columnID] = workspaceID
}

func (f *FakeTaskRepo) AddTaskFixture(id, columnID, title string, position int) *models.Task {
	task := &models.Task{ID: id, ColumnID: columnID, Title: title, Position: position, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.Tasks[id] = task
	return task
}

func (f *FakeTaskRepo) CreateTask(_ context.Context, task *models.Task) error {
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	f.Tasks[task.ID] = task
	return nil
}

func (f *FakeTaskRepo) FindTaskByID(_ context.Context, id string) (*models.Task, error) {
	task, ok := f.Tasks[id]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return task, nil
}

func (f *FakeTaskRepo) UpdateTask(_ context.Context, task *models.Task) error {
	existing, ok := f.Tasks[task.ID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	f.Tasks[task.ID] = &models.Task{
		ID: task.ID, ColumnID: task.ColumnID, Title: task.Title, Description: task.Description,
		AuthorID: task.AuthorID, AssigneeID: task.AssigneeID, CuratorID: task.CuratorID, ParentID: task.ParentID,
		DueDate: task.DueDate, Position: task.Position,
		IsUrgent: task.IsUrgent, IsHidden: task.IsHidden, ArchivedAt: task.ArchivedAt,
		CreatedAt: existing.CreatedAt, UpdatedAt: time.Now(),
	}
	return nil
}

func (f *FakeTaskRepo) DeleteTask(_ context.Context, id string) error {
	if _, ok := f.Tasks[id]; !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	delete(f.Tasks, id)
	return nil
}

func (f *FakeTaskRepo) ListTasksForBoard(_ context.Context, boardID string) ([]*models.Task, error) {
	var out []*models.Task
	for _, task := range f.Tasks {
		if f.ColumnBoards[task.ColumnID] == boardID {
			out = append(out, task)
		}
	}
	sortByPosition(out)
	return out, nil
}

func (f *FakeTaskRepo) ListTasksForColumn(_ context.Context, columnID string) ([]*models.Task, error) {
	var out []*models.Task
	for _, task := range f.Tasks {
		if task.ColumnID == columnID {
			out = append(out, task)
		}
	}
	sortByPosition(out)
	return out, nil
}

func (f *FakeTaskRepo) ListTasksForWorkspace(_ context.Context, workspaceID string) ([]*models.Task, error) {
	var out []*models.Task
	for _, task := range f.Tasks {
		if f.ColumnWorkspaces[task.ColumnID] == workspaceID {
			out = append(out, task)
		}
	}
	sortByPosition(out)
	return out, nil
}

func (f *FakeTaskRepo) FindTasksByIDs(_ context.Context, ids []string) (map[string]*models.Task, error) {
	result := make(map[string]*models.Task)
	for _, id := range ids {
		if task, ok := f.Tasks[id]; ok {
			result[id] = task
		}
	}
	return result, nil
}

func (f *FakeTaskRepo) ListChildTasks(_ context.Context, parentID string) ([]*models.Task, error) {
	var out []*models.Task
	for _, task := range f.Tasks {
		if task.ParentID == parentID {
			out = append(out, task)
		}
	}
	sortByPosition(out)
	return out, nil
}

func sortByPosition(tasks []*models.Task) {
	for i := 1; i < len(tasks); i++ {
		for j := i; j > 0 && tasks[j-1].Position > tasks[j].Position; j-- {
			tasks[j-1], tasks[j] = tasks[j], tasks[j-1]
		}
	}
}

var _ repository.TaskRepository = (*FakeTaskRepo)(nil)

type FakeHub struct {
	Messages []*ws.Message
}

func NewFakeHub() *FakeHub {
	return &FakeHub{}
}

func (f *FakeHub) Register(ws.Client)          {}
func (f *FakeHub) Unregister(ws.Client)        {}
func (f *FakeHub) JoinRoom(string, ws.Client)  {}
func (f *FakeHub) LeaveRoom(string, ws.Client) {}
func (f *FakeHub) RoomMembers(string) []string { return nil }
func (f *FakeHub) BroadcastToRoom(_ string, msg *ws.Message) {
	f.Messages = append(f.Messages, msg)
}
func (f *FakeHub) Broadcast(msg *ws.Message) {
	f.Messages = append(f.Messages, msg)
}
func (f *FakeHub) SendToUser(_ string, msg *ws.Message) {
	f.Messages = append(f.Messages, msg)
}
func (f *FakeHub) Close() {}

var _ ws.Hub = (*FakeHub)(nil)

func NewUUID() string {
	return uuid.New().String()
}

type FakeAttachmentRepo struct {
	Attachments map[string]models.TaskAttachment
}

func NewFakeAttachmentRepo() *FakeAttachmentRepo {
	return &FakeAttachmentRepo{Attachments: make(map[string]models.TaskAttachment)}
}

func (f *FakeAttachmentRepo) CreateAttachment(_ context.Context, attachment *models.TaskAttachment) error {
	attachment.CreatedAt = time.Now()
	f.Attachments[attachment.ID] = *attachment
	return nil
}

func (f *FakeAttachmentRepo) FindAttachmentByID(_ context.Context, id string) (*models.TaskAttachment, error) {
	a, ok := f.Attachments[id]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return &a, nil
}

func (f *FakeAttachmentRepo) ListAttachmentsByTask(_ context.Context, taskID string) ([]models.TaskAttachment, error) {
	var out []models.TaskAttachment
	for _, a := range f.Attachments {
		if a.TaskID == taskID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *FakeAttachmentRepo) DeleteAttachment(_ context.Context, id string) error {
	if _, ok := f.Attachments[id]; !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	delete(f.Attachments, id)
	return nil
}

var _ repository.AttachmentRepository = (*FakeAttachmentRepo)(nil)

type FakeFavoriteRepo struct {
	Favorites map[string]models.Favorite
}

func NewFakeFavoriteRepo() *FakeFavoriteRepo {
	return &FakeFavoriteRepo{Favorites: make(map[string]models.Favorite)}
}

func (f *FakeFavoriteRepo) AddFavorite(_ context.Context, favorite *models.Favorite) error {
	favorite.CreatedAt = time.Now()
	f.Favorites[favorite.UserID+"|"+favorite.TargetType+"|"+favorite.TargetID] = *favorite
	return nil
}

func (f *FakeFavoriteRepo) RemoveFavorite(_ context.Context, userID, targetType, targetID string) error {
	delete(f.Favorites, userID+"|"+targetType+"|"+targetID)
	return nil
}

func (f *FakeFavoriteRepo) IsFavorite(_ context.Context, userID, targetType, targetID string) (bool, error) {
	_, ok := f.Favorites[userID+"|"+targetType+"|"+targetID]
	return ok, nil
}

func (f *FakeFavoriteRepo) ListFavoriteTargets(_ context.Context, userID, targetType string) (map[string]bool, error) {
	result := make(map[string]bool)
	for k := range f.Favorites {
		parts := strings.Split(k, "|")
		if parts[0] == userID && parts[1] == targetType {
			result[parts[2]] = true
		}
	}
	return result, nil
}

var _ repository.FavoriteRepository = (*FakeFavoriteRepo)(nil)

type FakeCache struct {
	mu      sync.Mutex
	entries map[string]fakeCacheEntry
}

type fakeCacheEntry struct {
	value string
	exp   time.Time
}

func NewFakeCache() *FakeCache {
	return &FakeCache{entries: make(map[string]fakeCacheEntry)}
}

func (f *FakeCache) live() []string {
	var expired []string
	for k, e := range f.entries {
		if !e.exp.IsZero() && time.Now().After(e.exp) {
			expired = append(expired, k)
		}
	}
	return expired
}

func (f *FakeCache) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range f.live() {
		delete(f.entries, k)
	}
	e, ok := f.entries[key]
	if !ok {
		return "", nil
	}
	return e.value, nil
}

func (f *FakeCache) MGet(_ context.Context, keys ...string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range f.live() {
		delete(f.entries, k)
	}
	results := make([]string, len(keys))
	for i, key := range keys {
		if e, ok := f.entries[key]; ok {
			results[i] = e.value
		}
	}
	return results, nil
}

func (f *FakeCache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	f.entries[key] = fakeCacheEntry{value: value, exp: exp}
	return nil
}

func (f *FakeCache) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.entries, key)
	return nil
}

func (f *FakeCache) Expire(_ context.Context, key string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[key]
	if !ok {
		return nil
	}
	e.exp = time.Now().Add(ttl)
	f.entries[key] = e
	return nil
}

func (f *FakeCache) Incr(_ context.Context, key string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[key]
	next := int64(0)
	if ok && e.value != "" {
		if v, err := strconv.ParseInt(e.value, 10, 64); err == nil {
			next = v
		}
	}
	next++
	f.entries[key] = fakeCacheEntry{value: strconv.FormatInt(next, 10)}
	return next, nil
}

func (f *FakeCache) Ping(context.Context) error { return nil }
func (f *FakeCache) Close() error               { return nil }

var _ cache.Cache = (*FakeCache)(nil)
