package reviews

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/n0madic/google-play-scraper/internal/parse"
	"github.com/n0madic/google-play-scraper/internal/util"
	"github.com/n0madic/google-play-scraper/pkg/store"
)

const (
	initialRequest               = `f.req=%5B%5B%5B%22UsvDTd%22%2C%22%5Bnull%2Cnull%2C%5B2%2C{{sort}}%2C%5B{{maxNumberOfReviewsPerRequest}}%2Cnull%2Cnull%5D%2Cnull%2C%5B%5D%5D%2C%5B%5C%22{{appId}}%5C%22%2C7%5D%5D%22%2Cnull%2C%22generic%22%5D%5D%5D`
	paginatedRequest             = `f.req=%5B%5B%5B%22UsvDTd%22%2C%22%5Bnull%2Cnull%2C%5B2%2C{{sort}}%2C%5B{{maxNumberOfReviewsPerRequest}}%2Cnull%2C%5C%22{{withToken}}%5C%22%5D%2Cnull%2C%5B%5D%5D%2C%5B%5C%22{{appId}}%5C%22%2C7%5D%5D%22%2Cnull%2C%22generic%22%5D%5D%5D`
	maxNumberOfReviewsPerRequest = 150
)

// Options of reviews
type Options struct {
	Country  string
	Language string
	Number   int
	Sorting  store.Sort
}

// Review of app
type Review struct {
	Avatar         string
	Criteria       map[string]int64
	ID             string
	Score          int
	Reviewer       string
	Reply          string
	ReplyTimestamp time.Time
	Respondent     string
	Text           string
	Timestamp      time.Time
	Useful         int
	Version        string
}

// URL of review
func (r *Review) URL(appID string) string {
	if r.ID != "" {
		return fmt.Sprintf("https://play.google.com/store/apps/details?id=%s&reviewId=%s", appID, r.ID)
	}
	return ""
}

// Reviews instance
type Reviews struct {
	appID   string
	options *Options
}

// New return similar list instance
func New(appID string, options Options) *Reviews {
	if options.Number == 0 {
		options.Number = maxNumberOfReviewsPerRequest
	}
	if options.Sorting == 0 {
		options.Sorting = store.SortHelpfulness
	}
	return &Reviews{
		appID:   appID,
		options: &options,
	}
}

func (reviews *Reviews) batchexecute(payload string) ([]Review, string, error) {
	js, err := util.BatchExecute(reviews.options.Country, reviews.options.Language, payload)
	if err != nil {
		return nil, "", err
	}

	nextToken := util.GetJSONValue(js, "1.1")

	var results []Review
	rev := util.GetJSONArray(js, "0")
	for _, review := range rev {
		result := Parse(review.String())
		if result != nil {
			results = append(results, *result)
		}
	}

	return results, nextToken, nil
}

// RunPaging 分頁查詢
// resultFunc 如返回 true ,則不再獲取下一頁
func (reviews *Reviews) RunPaging(resultFunc func([]Review) (stop bool)) error {

	if reviews.options.Number > maxNumberOfReviewsPerRequest {
		reviews.options.Number = maxNumberOfReviewsPerRequest
	}

	r := strings.NewReplacer("{{sort}}", strconv.Itoa(int(reviews.options.Sorting)),
		"{{maxNumberOfReviewsPerRequest}}", strconv.Itoa(reviews.options.Number),
		"{{appId}}", reviews.appID,
	)
	payload := r.Replace(initialRequest)
	for {
		results, token, err := reviews.batchexecute(payload)
		if err != nil {
			return err
		}

		if resultFunc(results) {
			return nil
		}
		if token == "" || len(results) == 0 {
			return nil
		}
		r = strings.NewReplacer("{{sort}}", strconv.Itoa(int(reviews.options.Sorting)),
			"{{maxNumberOfReviewsPerRequest}}", strconv.Itoa(reviews.options.Number),
			"{{withToken}}", token,
			"{{appId}}", reviews.appID,
		)
		payload = r.Replace(paginatedRequest)
	}
}

// Parse app review
func Parse(review string) *Review {
	text := util.GetJSONValue(review, "4")
	if text != "" {
		criteriaList := util.GetJSONArray(review, "12.0")
		criteria := make(map[string]int64, len(criteriaList))
		for _, criterion := range criteriaList {
			var rating int64
			if len(criterion.Array()) > 2 {
				rating = criterion.Array()[2].Array()[0].Int()
			}
			criteria[criterion.Array()[0].String()] = rating
		}
		return &Review{
			Avatar:         util.GetJSONValue(review, "1.1.3.2"),
			Criteria:       criteria,
			ID:             util.GetJSONValue(review, "0"),
			Reply:          util.GetJSONValue(review, "7.1"),
			ReplyTimestamp: time.Unix(parse.Int64(util.GetJSONValue(review, "7.2.0")), 0),
			Respondent:     util.GetJSONValue(review, "7.0"),
			Reviewer:       util.GetJSONValue(review, "1.0"),
			Score:          parse.Int(util.GetJSONValue(review, "2")),
			Text:           text,
			Timestamp:      time.Unix(parse.Int64(util.GetJSONValue(review, "5.0")), 0),
			Useful:         parse.Int(util.GetJSONValue(review, "6")),
			Version:        util.GetJSONValue(review, "10"),
		}
	}
	return nil
}
