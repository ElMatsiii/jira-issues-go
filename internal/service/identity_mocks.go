package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"jira-issues-go/internal/domain"
)

// MockIdentityRepository is a thread-safe in-memory IdentityRepository for tests.
type MockIdentityRepository struct {
	mu         sync.Mutex
	Users      map[string]*domain.User         // by id
	Workspaces map[string]*domain.Workspace    // by id (Role unused)
	Members    map[string]domain.WorkspaceRole // by workspaceID + "/" + userID
	Projects   map[string]*domain.Project      // by id
	// ProjectMembers holds the invited members, by projectID + "/" + userID.
	ProjectMembers map[string]bool
	// Err, when set, is returned by every method (simulates a database failure).
	Err   error
	clock time.Time
}

func NewMockIdentityRepository() *MockIdentityRepository {
	return &MockIdentityRepository{
		Users:          make(map[string]*domain.User),
		Workspaces:     make(map[string]*domain.Workspace),
		Members:        make(map[string]domain.WorkspaceRole),
		Projects:       make(map[string]*domain.Project),
		ProjectMembers: make(map[string]bool),
		clock:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// tick returns strictly increasing timestamps so ordering by creation time is deterministic.
func (m *MockIdentityRepository) tick() time.Time {
	m.clock = m.clock.Add(time.Minute)
	return m.clock
}

func memberKey(workspaceID, userID string) string { return workspaceID + "/" + userID }

// AddWorkspace seeds a workspace with one member (test helper).
func (m *MockIdentityRepository) AddWorkspace(workspaceID, userID string, role domain.WorkspaceRole) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.Workspaces[workspaceID]; !exists {
		m.Workspaces[workspaceID] = &domain.Workspace{ID: workspaceID, Name: "Espacio " + workspaceID[:4], CreatedAt: m.tick()}
	}
	m.Members[memberKey(workspaceID, userID)] = role
}

func (m *MockIdentityRepository) CreateUserWithWorkspace(ctx context.Context, user *domain.User, workspaceName string) (*domain.Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	for _, existing := range m.Users {
		if existing.Email == user.Email {
			return nil, domain.ErrEmailTaken
		}
	}
	user.ID = uuid.NewString()
	user.CreatedAt = m.tick()
	stored := *user
	m.Users[user.ID] = &stored
	workspace := &domain.Workspace{ID: uuid.NewString(), Name: workspaceName, CreatedAt: m.tick()}
	m.Workspaces[workspace.ID] = workspace
	m.Members[memberKey(workspace.ID, user.ID)] = domain.RoleOwner
	return &domain.Workspace{ID: workspace.ID, Name: workspace.Name, Role: domain.RoleOwner, CreatedAt: workspace.CreatedAt}, nil
}

func (m *MockIdentityRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	for _, user := range m.Users {
		if user.Email == email {
			copied := *user
			return &copied, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (m *MockIdentityRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	user, exists := m.Users[id]
	if !exists {
		return nil, domain.ErrUserNotFound
	}
	copied := *user
	return &copied, nil
}

func (m *MockIdentityRepository) ListUserWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	workspaces := []*domain.Workspace{}
	for id, workspace := range m.Workspaces {
		if role, member := m.Members[memberKey(id, userID)]; member {
			workspaces = append(workspaces, &domain.Workspace{ID: id, Name: workspace.Name, Role: role, CreatedAt: workspace.CreatedAt})
		}
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].CreatedAt.Before(workspaces[j].CreatedAt) })
	return workspaces, nil
}

func (m *MockIdentityRepository) GetMemberRole(ctx context.Context, workspaceID, userID string) (domain.WorkspaceRole, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return domain.RoleNone, m.Err
	}
	return m.Members[memberKey(workspaceID, userID)], nil
}

func (m *MockIdentityRepository) AddMember(ctx context.Context, workspaceID, userID string, role domain.WorkspaceRole) (domain.WorkspaceRole, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return domain.RoleNone, m.Err
	}
	key := memberKey(workspaceID, userID)
	if existing, member := m.Members[key]; member {
		return existing, nil
	}
	m.Members[key] = role
	return role, nil
}

func (m *MockIdentityRepository) CreateWorkspaceProject(ctx context.Context, project *domain.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	for _, existing := range m.Projects {
		if existing.Key == project.Key {
			return domain.ErrProjectKeyTaken
		}
	}
	project.ID = uuid.NewString()
	project.CreatedAt = m.tick()
	project.UpdatedAt = project.CreatedAt
	stored := *project
	m.Projects[project.ID] = &stored
	return nil
}

func (m *MockIdentityRepository) ListWorkspaceProjects(ctx context.Context, workspaceID string) ([]*domain.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	projects := []*domain.Project{}
	for _, project := range m.Projects {
		if project.WorkspaceID == workspaceID {
			copied := *project
			projects = append(projects, &copied)
		}
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].CreatedAt.After(projects[j].CreatedAt) })
	return projects, nil
}

func (m *MockIdentityRepository) GetProjectWorkspaceID(ctx context.Context, projectID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return "", m.Err
	}
	project, exists := m.Projects[projectID]
	if !exists || project.WorkspaceID == "" {
		return "", domain.ErrProjectNotFound
	}
	return project.WorkspaceID, nil
}

func (m *MockIdentityRepository) ListMemberProjects(ctx context.Context, workspaceID, userID string) ([]*domain.Project, error) {
	all, err := m.ListWorkspaceProjects(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	projects := []*domain.Project{}
	for _, project := range all {
		if m.ProjectMembers[memberKey(project.ID, userID)] {
			projects = append(projects, project)
		}
	}
	return projects, nil
}

func (m *MockIdentityRepository) IsProjectMember(ctx context.Context, projectID, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return false, m.Err
	}
	return m.ProjectMembers[memberKey(projectID, userID)], nil
}

func (m *MockIdentityRepository) AddProjectMember(ctx context.Context, projectID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.ProjectMembers[memberKey(projectID, userID)] = true
	return nil
}

func (m *MockIdentityRepository) RemoveProjectMember(ctx context.Context, projectID, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return false, m.Err
	}
	key := memberKey(projectID, userID)
	removed := m.ProjectMembers[key]
	delete(m.ProjectMembers, key)
	return removed, nil
}

func (m *MockIdentityRepository) ListProjectMembers(ctx context.Context, projectID, workspaceID string) ([]*domain.ProjectMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	order := map[domain.WorkspaceRole]int{domain.RoleOwner: 0, domain.RoleAdmin: 1, domain.RoleMember: 2}
	members := []*domain.ProjectMember{}
	for userID, user := range m.Users {
		role, inWorkspace := m.Members[memberKey(workspaceID, userID)]
		if !inWorkspace || (!role.ReachesEveryProject() && !m.ProjectMembers[memberKey(projectID, userID)]) {
			continue
		}
		copied := *user
		copied.PasswordHash = ""
		members = append(members, &domain.ProjectMember{User: &copied, Role: role, Invited: !role.ReachesEveryProject()})
	}
	sort.Slice(members, func(i, j int) bool {
		if order[members[i].Role] != order[members[j].Role] {
			return order[members[i].Role] < order[members[j].Role]
		}
		return members[i].User.Name < members[j].User.Name
	})
	return members, nil
}
