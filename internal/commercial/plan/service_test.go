// Package plan provides plan management functionality.
package plan

import (
	"context"
	"testing"

	"v/internal/database/repository"
	"v/internal/logger"
)

// mockPlanRepository is a mock implementation of PlanRepository for testing.
type mockPlanRepository struct {
	plans  map[int64]*repository.CommercialPlan
	groups map[int64]*repository.PlanGroup
	nextID int64
}

func newMockPlanRepository() *mockPlanRepository {
	return &mockPlanRepository{
		plans:  make(map[int64]*repository.CommercialPlan),
		groups: make(map[int64]*repository.PlanGroup),
		nextID: 1,
	}
}

func (m *mockPlanRepository) Create(ctx context.Context, plan *repository.CommercialPlan) error {
	plan.ID = m.nextID
	m.nextID++
	m.plans[plan.ID] = plan
	return nil
}

func (m *mockPlanRepository) GetByID(ctx context.Context, id int64) (*repository.CommercialPlan, error) {
	if plan, ok := m.plans[id]; ok {
		return plan, nil
	}
	return nil, ErrPlanNotFound
}

func (m *mockPlanRepository) Update(ctx context.Context, plan *repository.CommercialPlan) error {
	if _, ok := m.plans[plan.ID]; !ok {
		return ErrPlanNotFound
	}
	m.plans[plan.ID] = plan
	return nil
}

func (m *mockPlanRepository) Delete(ctx context.Context, id int64) error {
	delete(m.plans, id)
	return nil
}

func (m *mockPlanRepository) List(ctx context.Context, filter repository.PlanFilter, limit, offset int) ([]*repository.CommercialPlan, int64, error) {
	var result []*repository.CommercialPlan
	for _, p := range m.plans {
		if filter.IsActive != nil && p.IsActive != *filter.IsActive {
			continue
		}
		result = append(result, p)
	}
	return result, int64(len(result)), nil
}

func (m *mockPlanRepository) ListActive(ctx context.Context) ([]*repository.CommercialPlan, error) {
	var result []*repository.CommercialPlan
	for _, p := range m.plans {
		if p.IsActive {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPlanRepository) SetActive(ctx context.Context, id int64, active bool) error {
	if plan, ok := m.plans[id]; ok {
		plan.IsActive = active
		return nil
	}
	return ErrPlanNotFound
}

func (m *mockPlanRepository) Count(ctx context.Context) (int64, error) {
	return int64(len(m.plans)), nil
}

func (m *mockPlanRepository) CountActive(ctx context.Context) (int64, error) {
	var count int64
	for _, p := range m.plans {
		if p.IsActive {
			count++
		}
	}
	return count, nil
}

func (m *mockPlanRepository) CreateGroup(ctx context.Context, group *repository.PlanGroup) error {
	group.ID = m.nextID
	m.nextID++
	m.groups[group.ID] = group
	return nil
}

func (m *mockPlanRepository) GetGroupByID(ctx context.Context, id int64) (*repository.PlanGroup, error) {
	if group, ok := m.groups[id]; ok {
		return group, nil
	}
	return nil, ErrGroupNotFound
}

func (m *mockPlanRepository) UpdateGroup(ctx context.Context, group *repository.PlanGroup) error {
	if _, ok := m.groups[group.ID]; !ok {
		return ErrGroupNotFound
	}
	m.groups[group.ID] = group
	return nil
}

func (m *mockPlanRepository) DeleteGroup(ctx context.Context, id int64) error {
	delete(m.groups, id)
	return nil
}

func (m *mockPlanRepository) ListGroups(ctx context.Context) ([]*repository.PlanGroup, error) {
	var result []*repository.PlanGroup
	for _, g := range m.groups {
		result = append(result, g)
	}
	return result, nil
}

func TestService_Create(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)

	ctx := context.Background()

	// Test successful creation
	req := &CreatePlanRequest{
		Name:        "Basic Plan",
		Description: "A basic plan",
		Duration:    30,
		Price:       1000,
		IsActive:    true,
	}

	plan, err := svc.Create(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if plan.Name != "Basic Plan" {
		t.Errorf("Expected name 'Basic Plan', got '%s'", plan.Name)
	}
	if plan.Duration != 30 {
		t.Errorf("Expected duration 30, got %d", plan.Duration)
	}
	if plan.Price != 1000 {
		t.Errorf("Expected price 1000, got %d", plan.Price)
	}
	if plan.PlanType != "monthly" {
		t.Errorf("Expected default plan type 'monthly', got '%s'", plan.PlanType)
	}
}

func TestService_Create_ValidationErrors(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)

	ctx := context.Background()

	// Test empty name
	_, err := svc.Create(ctx, &CreatePlanRequest{
		Duration: 30,
		Price:    1000,
	})
	if err == nil {
		t.Error("Expected error for empty name")
	}

	// Test zero duration
	_, err = svc.Create(ctx, &CreatePlanRequest{
		Name:     "Test",
		Duration: 0,
		Price:    1000,
	})
	if err == nil {
		t.Error("Expected error for zero duration")
	}

	// Test negative price
	_, err = svc.Create(ctx, &CreatePlanRequest{
		Name:     "Test",
		Duration: 30,
		Price:    -100,
	})
	if err == nil {
		t.Error("Expected error for negative price")
	}
}

func TestService_Update(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)

	ctx := context.Background()

	// Create a plan first
	plan, _ := svc.Create(ctx, &CreatePlanRequest{
		Name:     "Original",
		Duration: 30,
		Price:    1000,
		IsActive: true,
	})

	// Update the plan
	newName := "Updated"
	newPrice := int64(2000)
	updated, err := svc.Update(ctx, plan.ID, &UpdatePlanRequest{
		Name:  &newName,
		Price: &newPrice,
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("Expected name 'Updated', got '%s'", updated.Name)
	}
	if updated.Price != 2000 {
		t.Errorf("Expected price 2000, got %d", updated.Price)
	}
}

func TestService_Update_ValidationErrors(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)
	ctx := context.Background()

	created, err := svc.Create(ctx, &CreatePlanRequest{
		Name:     "Valid Plan",
		Duration: 30,
		Price:    1000,
	})
	if err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	negativePrice := int64(-1)
	if _, err := svc.Update(ctx, created.ID, &UpdatePlanRequest{Price: &negativePrice}); err == nil {
		t.Fatal("expected negative price update to fail")
	}

	zeroDuration := 0
	if _, err := svc.Update(ctx, created.ID, &UpdatePlanRequest{Duration: &zeroDuration}); err == nil {
		t.Fatal("expected zero duration update to fail")
	}

	negativeTraffic := int64(-1024)
	if _, err := svc.Update(ctx, created.ID, &UpdatePlanRequest{TrafficLimit: &negativeTraffic}); err == nil {
		t.Fatal("expected negative traffic limit update to fail")
	}

	negativeIPLimit := -1
	if _, err := svc.Update(ctx, created.ID, &UpdatePlanRequest{IPLimit: &negativeIPLimit}); err == nil {
		t.Fatal("expected negative ip limit update to fail")
	}

	invalidPlanType := "weekly"
	if _, err := svc.Update(ctx, created.ID, &UpdatePlanRequest{PlanType: &invalidPlanType}); err == nil {
		t.Fatal("expected invalid plan type update to fail")
	}

	invalidResetCycle := "daily"
	if _, err := svc.Update(ctx, created.ID, &UpdatePlanRequest{ResetCycle: &invalidResetCycle}); err == nil {
		t.Fatal("expected invalid reset cycle update to fail")
	}
}

func TestService_Create_RejectsInvalidPlanOptions(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)
	ctx := context.Background()

	if _, err := svc.Create(ctx, &CreatePlanRequest{
		Name:     "Bad Type",
		Duration: 30,
		Price:    1000,
		PlanType: "weekly",
	}); err == nil {
		t.Fatal("expected invalid plan type to fail")
	}

	if _, err := svc.Create(ctx, &CreatePlanRequest{
		Name:       "Bad Reset",
		Duration:   30,
		Price:      1000,
		ResetCycle: "daily",
	}); err == nil {
		t.Fatal("expected invalid reset cycle to fail")
	}
}

func TestService_SetActive(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)

	ctx := context.Background()

	// Create an active plan
	plan, _ := svc.Create(ctx, &CreatePlanRequest{
		Name:     "Test",
		Duration: 30,
		Price:    1000,
		IsActive: true,
	})

	// Disable the plan
	err := svc.SetActive(ctx, plan.ID, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify it's disabled
	updated, _ := svc.GetByID(ctx, plan.ID)
	if updated.IsActive {
		t.Error("Expected plan to be inactive")
	}

	// Re-enable the plan
	err = svc.SetActive(ctx, plan.ID, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify it's enabled
	updated, _ = svc.GetByID(ctx, plan.ID)
	if !updated.IsActive {
		t.Error("Expected plan to be active")
	}
}

func TestService_Delete(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)

	ctx := context.Background()

	// Create a plan
	plan, _ := svc.Create(ctx, &CreatePlanRequest{
		Name:     "Test",
		Duration: 30,
		Price:    1000,
	})

	// Delete the plan
	err := svc.Delete(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify it's deleted
	_, err = svc.GetByID(ctx, plan.ID)
	if err != ErrPlanNotFound {
		t.Error("Expected ErrPlanNotFound after deletion")
	}
}

func TestService_ListActive(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)

	ctx := context.Background()

	// Create active and inactive plans
	svc.Create(ctx, &CreatePlanRequest{
		Name:     "Active 1",
		Duration: 30,
		Price:    1000,
		IsActive: true,
	})
	svc.Create(ctx, &CreatePlanRequest{
		Name:     "Inactive",
		Duration: 30,
		Price:    1000,
		IsActive: false,
	})
	svc.Create(ctx, &CreatePlanRequest{
		Name:     "Active 2",
		Duration: 30,
		Price:    2000,
		IsActive: true,
	})

	// List active plans
	plans, err := svc.ListActive(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(plans) != 2 {
		t.Errorf("Expected 2 active plans, got %d", len(plans))
	}
}

func TestService_CalculateMonthlyPrice(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		name     string
		price    int64
		duration int
		expected int64
	}{
		{"30 days", 1000, 30, 1000},
		{"90 days", 2700, 90, 900},
		{"365 days", 10950, 365, 900},
		{"Zero duration", 1000, 0, 0},
		{"Negative duration", 1000, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &Plan{Price: tt.price, Duration: tt.duration}
			result := svc.CalculateMonthlyPrice(plan)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestService_Groups(t *testing.T) {
	repo := newMockPlanRepository()
	log := logger.NewNopLogger()
	svc := NewService(repo, log)

	ctx := context.Background()

	// Create a group
	group, err := svc.CreateGroup(ctx, "Premium Plans", 1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if group.Name != "Premium Plans" {
		t.Errorf("Expected name 'Premium Plans', got '%s'", group.Name)
	}

	// Update the group
	updated, err := svc.UpdateGroup(ctx, group.ID, "VIP Plans", 2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if updated.Name != "VIP Plans" {
		t.Errorf("Expected name 'VIP Plans', got '%s'", updated.Name)
	}

	// List groups
	groups, err := svc.ListGroups(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(groups))
	}

	// Delete the group
	err = svc.DeleteGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify deletion
	_, err = svc.GetGroupByID(ctx, group.ID)
	if err != ErrGroupNotFound {
		t.Error("Expected ErrGroupNotFound after deletion")
	}
}
