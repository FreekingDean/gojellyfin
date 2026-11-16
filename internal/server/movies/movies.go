package movies

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

const (
	categories    = 5
	perCategory   = 8
	recommendable = items.Kind(api.BaseItemKindMovie)
)

type Server struct {
	items    *items.Service
	policies dto.Access
}

func New(items *items.Service, policies dto.Access) *Server {
	return &Server{items: items, policies: policies}
}

func (s *Server) GetMovieRecommendations(ctx context.Context, request api.GetMovieRecommendationsRequestObject) (api.GetMovieRecommendationsResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	played, err := s.items.RecentlyPlayed(ctx, viewer, auth.UserID(ctx), recommendable, int(apiutil.OrElse(request.Params.CategoryLimit, categories)))
	if err != nil {
		return nil, err
	}

	recommendations := make(api.GetMovieRecommendations200JSONResponse, 0, len(played))
	for _, baseline := range played {
		similar, err := s.items.Similar(ctx, viewer, baseline.ID, int(apiutil.OrElse(request.Params.ItemLimit, perCategory)))
		if err != nil {
			return nil, err
		}
		if len(similar) == 0 {
			continue
		}

		converted, err := dto.ItemDtos(ctx, s.items, similar)
		if err != nil {
			return nil, err
		}

		recommendations = append(recommendations, api.RecommendationDto{
			BaselineItemName:   apiutil.Ptr(baseline.Name),
			CategoryId:         apiutil.Ptr(baseline.ID),
			Items:              &converted,
			RecommendationType: apiutil.Ptr(api.SimilarToRecentlyPlayed),
		})
	}

	return recommendations, nil
}
