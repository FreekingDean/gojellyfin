package activitylog

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/server/activitylog/mocks"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestServer_GetLogEntries(t *testing.T) {
	at := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	itemID := uuid.New()
	failed := errors.New("the database is gone")

	tests := []struct {
		name             string
		request          api.GetLogEntriesRequestObject
		expectedResponse api.GetLogEntriesResponseObject
		expectedError    error

		mockExpectedQuery activity.Query
		mockResponse      []*activity.Entry
		mockTotal         int
		mockError         error
	}{
		{
			name:    "empty result",
			request: api.GetLogEntriesRequestObject{},
			expectedResponse: api.GetLogEntries200JSONResponse{
				Items:            &[]api.ActivityLogEntry{},
				StartIndex:       apiutil.Ptr(int32(0)),
				TotalRecordCount: apiutil.Ptr(int32(0)),
			},
			mockResponse: []*activity.Entry{},
		},
		{
			name: "passes the window and the page through",
			request: api.GetLogEntriesRequestObject{Params: api.GetLogEntriesParams{
				StartIndex: apiutil.Ptr(int32(2)),
				Limit:      apiutil.Ptr(int32(5)),
				MinDate:    &at,
				MaxDate:    &at,
				HasUserId:  apiutil.Ptr(true),
			}},
			expectedResponse: api.GetLogEntries200JSONResponse{
				Items:            &[]api.ActivityLogEntry{},
				StartIndex:       apiutil.Ptr(int32(2)),
				TotalRecordCount: apiutil.Ptr(int32(9)),
			},
			mockExpectedQuery: activity.Query{
				StartIndex: 2,
				Limit:      5,
				MinDate:    &at,
				MaxDate:    &at,
				HasUserID:  apiutil.Ptr(true),
			},
			mockResponse: []*activity.Entry{},
			mockTotal:    9,
		},
		{
			name:    "translates an entry",
			request: api.GetLogEntriesRequestObject{},
			expectedResponse: api.GetLogEntries200JSONResponse{
				Items: &[]api.ActivityLogEntry{
					{
						Date:          apiutil.Ptr(at),
						Name:          apiutil.Ptr("Scan"),
						Type:          apiutil.Ptr(activity.KindLibraryScanCompleted),
						Severity:      apiutil.Ptr(api.LogLevel(activity.SeverityInformation)),
						UserId:        &userID,
						ItemId:        apiutil.Ptr(itemID.String()),
						Overview:      apiutil.Ptr("overview"),
						ShortOverview: apiutil.Ptr("short"),
					},
				},
				StartIndex:       apiutil.Ptr(int32(0)),
				TotalRecordCount: apiutil.Ptr(int32(1)),
			},
			mockResponse: []*activity.Entry{
				{
					CreatedAt:     at,
					Name:          "Scan",
					Kind:          activity.KindLibraryScanCompleted,
					Severity:      activity.SeverityInformation,
					UserID:        &userID,
					ItemID:        &itemID,
					Overview:      "overview",
					ShortOverview: "short",
				},
			},
			mockTotal: 1,
		},
		{
			name:    "leaves an empty overview out",
			request: api.GetLogEntriesRequestObject{},
			expectedResponse: api.GetLogEntries200JSONResponse{
				Items: &[]api.ActivityLogEntry{
					{
						Date:     apiutil.Ptr(time.Time{}),
						Name:     apiutil.Ptr(""),
						Type:     apiutil.Ptr(""),
						Severity: apiutil.Ptr(api.LogLevel("")),
					},
				},
				StartIndex:       apiutil.Ptr(int32(0)),
				TotalRecordCount: apiutil.Ptr(int32(1)),
			},
			mockResponse: []*activity.Entry{{}},
			mockTotal:    1,
		},
		{
			name:          "returns the error",
			request:       api.GetLogEntriesRequestObject{},
			expectedError: failed,
			mockError:     failed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			activities := mocks.NewMockActivitiesService(ctrl)
			activities.EXPECT().
				Entries(gomock.Any(), test.mockExpectedQuery).
				Return(test.mockResponse, test.mockTotal, test.mockError)
			server := Server{
				activity: activities,
			}

			resp, err := server.GetLogEntries(t.Context(), test.request)
			assert.Equal(t, test.expectedError, err)
			assert.Equal(t, test.expectedResponse, resp)
		})
	}
}
