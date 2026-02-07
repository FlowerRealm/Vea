package nodegroups

import (
	"context"
	"fmt"
	"strings"

	"vea/backend/domain"
	"vea/backend/repository"
)

// Service NodeGroup 服务
type Service struct {
	repo repository.NodeGroupRepository
}

func NewService(repo repository.NodeGroupRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]domain.NodeGroup, error) {
	return s.repo.List(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (domain.NodeGroup, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, group domain.NodeGroup) (domain.NodeGroup, error) {
	group, err := normalizeNodeGroupForWrite(group)
	if err != nil {
		return domain.NodeGroup{}, err
	}
	return s.repo.Create(ctx, group)
}

func (s *Service) Update(ctx context.Context, id string, updateFn func(domain.NodeGroup) (domain.NodeGroup, error)) (domain.NodeGroup, error) {
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.NodeGroup{}, err
	}
	next, err := updateFn(current)
	if err != nil {
		return domain.NodeGroup{}, err
	}
	next, err = normalizeNodeGroupForWrite(next)
	if err != nil {
		return domain.NodeGroup{}, err
	}
	next.Cursor = current.Cursor
	return s.repo.Update(ctx, id, next)
}

func (s *Service) UpdateCursor(ctx context.Context, id string, cursor int) error {
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	current.Cursor = cursor
	_, err = s.repo.Update(ctx, id, current)
	return err
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func normalizeNodeGroupForWrite(group domain.NodeGroup) (domain.NodeGroup, error) {
	if strings.TrimSpace(group.Name) == "" {
		return domain.NodeGroup{}, fmt.Errorf("%w: node group name is required", repository.ErrInvalidData)
	}

	switch group.Strategy {
	case domain.NodeGroupStrategyLowestLatency,
		domain.NodeGroupStrategyFastestSpeed,
		domain.NodeGroupStrategyRoundRobin,
		domain.NodeGroupStrategyFailover:
	default:
		return domain.NodeGroup{}, fmt.Errorf("%w: invalid node group strategy: %s", repository.ErrInvalidData, group.Strategy)
	}

	if group.Cursor < 0 {
		group.Cursor = 0
	}

	if group.NodeIDs != nil {
		out := make([]string, 0, len(group.NodeIDs))
		seen := make(map[string]struct{}, len(group.NodeIDs))
		for _, id := range group.NodeIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
		group.NodeIDs = out
	}

	if len(group.NodeIDs) == 0 {
		return domain.NodeGroup{}, fmt.Errorf("%w: node group must contain at least one node", repository.ErrInvalidData)
	}

	if group.Tags != nil {
		out := make([]string, 0, len(group.Tags))
		seen := make(map[string]struct{}, len(group.Tags))
		for _, tag := range group.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
		group.Tags = out
	}

	return group, nil
}
