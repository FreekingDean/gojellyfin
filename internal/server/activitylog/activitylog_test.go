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

	serverGetLogEntriesRest(t, at, itemID, userID, failed)
}

func serverGetLogEntriesRest(t *testing.T, at time.Time, itemID uuid.UUID, userID uuid.UUID, failed error) {
	t.Helper()

	tests := []serverGetLogEntriesCase{
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
			name: "passes the filters and the sort through",
			request: api.GetLogEntriesRequestObject{Params: api.GetLogEntriesParams{
				Name:          apiutil.Ptr("scan"),
				Overview:      apiutil.Ptr("over"),
				ShortOverview: apiutil.Ptr("short"),
				Type:          apiutil.Ptr("Library"),
				Username:      apiutil.Ptr("dean"),
				ItemId:        &itemID,
				Severity:      apiutil.Ptr(api.LogLevelWarning),
				SortBy: &[]api.ActivityLogSortBy{
					api.ActivityLogSortByLogSeverity,
					"Unknown",
					api.ActivityLogSortByUsername,
				},
				SortOrder: &[]api.SortOrder{api.Descending},
			}},
			expectedResponse: api.GetLogEntries200JSONResponse{
				Items:            &[]api.ActivityLogEntry{},
				StartIndex:       apiutil.Ptr(int32(0)),
				TotalRecordCount: apiutil.Ptr(int32(0)),
			},
			mockExpectedQuery: activity.Query{
				Name:          "scan",
				Overview:      "over",
				ShortOverview: "short",
				Kind:          "Library",
				Username:      "dean",
				ItemID:        &itemID,
				Severity:      apiutil.Ptr(activity.Severity("Warning")),
				Sort: []activity.Sort{
					{Field: activity.SortBySeverity, Descending: true},
					{Field: activity.SortByUsername},
				},
			},
			mockResponse: []*activity.Entry{},
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
		t.Run(test.name, testServer_GetLogEntriesCase(test))
	}
}

func testServer_GetLogEntriesCase(test serverGetLogEntriesCase) func(*testing.T) {
	return func(t *testing.T) {
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
	}
}

type serverGetLogEntriesCase struct {
	name             string
	request          api.GetLogEntriesRequestObject
	expectedResponse api.GetLogEntriesResponseObject
	expectedError    error

	mockExpectedQuery activity.Query
	mockResponse      []*activity.Entry
	mockTotal         int
	mockError         error
}
