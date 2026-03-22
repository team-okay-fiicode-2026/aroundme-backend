package http

import (
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/aroundme/aroundme-backend/internal/model"
	platformstorage "github.com/aroundme/aroundme-backend/internal/platform/storage"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

type PostHandler struct {
	authUseCase usecase.AuthUseCase
	imageStore  PostImageStore
	postUseCase usecase.PostUseCase
	streamHub   *PostStreamHub
}

type PostImageStore interface {
	Delete(publicPath string) error
	Save(file *multipart.FileHeader) (string, error)
}

type listPostsResponse struct {
	Items      []postSummaryResponse `json:"items"`
	NextCursor string                `json:"nextCursor"`
}

type postSummaryResponse struct {
	ID               string                   `json:"id"`
	Title            string                   `json:"title"`
	Excerpt          string                   `json:"excerpt"`
	Kind             string                   `json:"kind"`
	Status           string                   `json:"status"`
	Author           postAuthorResponse       `json:"author"`
	LocationName     *string                  `json:"locationName,omitempty"`
	Coordinates      *postCoordinatesResponse `json:"coordinates,omitempty"`
	IsLocationShared bool                     `json:"isLocationShared"`
	DistanceKm       *float64                 `json:"distanceKm"`
	ReactionCount    int                      `json:"reactionCount"`
	CommentCount     int                      `json:"commentCount"`
	IsReacted        bool                     `json:"isReacted"`
	Tags             []string                 `json:"tags"`
	ImageURL         string                   `json:"imageUrl,omitempty"`
	CreatedAt        string                   `json:"createdAt"`
}

type postDetailResponse struct {
	postSummaryResponse
	Body    string `json:"body"`
	IsOwner bool   `json:"isOwner"`
}

type postAuthorResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type postCoordinatesResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type postCommentResponse struct {
	ID            string                `json:"id"`
	ParentID      *string               `json:"parentId,omitempty"`
	Author        postAuthorResponse    `json:"author"`
	Body          string                `json:"body"`
	ReactionCount int                   `json:"reactionCount"`
	IsReacted     bool                  `json:"isReacted"`
	ReplyCount    int                   `json:"replyCount"`
	Replies       []postCommentResponse `json:"replies,omitempty"`
	CreatedAt     string                `json:"createdAt"`
	UpdatedAt     string                `json:"updatedAt"`
}

type listCommentsResponse struct {
	Items      []postCommentResponse `json:"items"`
	NextCursor string                `json:"nextCursor"`
}

type createPostRequest struct {
	Kind          string   `json:"kind"`
	Category      string   `json:"category"`
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	LocationName  string   `json:"locationName"`
	Latitude      float64  `json:"latitude"`
	Longitude     float64  `json:"longitude"`
	ShareLocation *bool    `json:"shareLocation"`
	Tags          []string `json:"tags"`
	ImageURL      string   `json:"imageUrl"`
}

type createCommentRequest struct {
	ParentID *string `json:"parentId"`
	Body     string  `json:"body"`
}

type updatePostStatusRequest struct {
	Status        string   `json:"status"`
	HelperUserIDs []string `json:"helperUserIds"`
}

type toggleReactionResponse struct {
	PostID        string `json:"postId"`
	ReactionCount int    `json:"reactionCount"`
	IsReacted     bool   `json:"isReacted"`
}

type createCommentResponse struct {
	Comment      postCommentResponse `json:"comment"`
	CommentCount int                 `json:"commentCount"`
}

func NewPostHandler(
	postUseCase usecase.PostUseCase,
	authUseCase usecase.AuthUseCase,
	streamHub *PostStreamHub,
	imageStore PostImageStore,
) *PostHandler {
	return &PostHandler{
		authUseCase: authUseCase,
		imageStore:  imageStore,
		postUseCase: postUseCase,
		streamHub:   streamHub,
	}
}

func (h *PostHandler) Register(app fiber.Router) {
	app.Use("/stream", h.authorizeStreamUpgrade)
	app.Get("/stream", websocket.New(h.stream))

	app.Get("/", AuthRequired(h.authUseCase), h.listPosts)
	app.Post("/", AuthRequired(h.authUseCase), h.createPost)
	app.Get("/:id/comments", AuthRequired(h.authUseCase), h.listComments)
	app.Post("/:id/comments", AuthRequired(h.authUseCase), h.createComment)
	app.Post("/:id/reactions/toggle", AuthRequired(h.authUseCase), h.toggleReaction)
	app.Post("/comments/:commentId/reactions/toggle", AuthRequired(h.authUseCase), h.toggleCommentReaction)
	app.Post("/:id/status", AuthRequired(h.authUseCase), h.updateStatus)
	app.Get("/:id", AuthRequired(h.authUseCase), h.getPost)
}

func (h *PostHandler) listPosts(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	distanceKm, err := parseOptionalFloatQuery(c, "distanceKm")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	var authorID *string
	if v := c.Query("authorId"); v != "" {
		authorID = &v
	}

	result, err := h.postUseCase.ListPosts(ctx, user.ID, model.ListPostsInput{
		AuthorID:   authorID,
		DistanceKm: distanceKm,
		Kind:       firstNonEmpty(c.Query("category"), c.Query("kind")),
		Status:     c.Query("status"),
		Cursor:     c.Query("cursor"),
		Limit:      parseOptionalIntQuery(c, "limit"),
	})
	if err != nil {
		return writePostError(c, err)
	}

	items := make([]postSummaryResponse, len(result.Items))
	for i, item := range result.Items {
		items[i] = presentPostSummary(item)
	}

	return c.JSON(listPostsResponse{
		Items:      items,
		NextCursor: result.NextCursor,
	})
}

func (h *PostHandler) getPost(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	postID := c.Params("id")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	post, err := h.postUseCase.GetPost(ctx, user.ID, postID)
	if err != nil {
		return writePostError(c, err)
	}

	return c.JSON(presentPostDetail(post))
}

func (h *PostHandler) createPost(c *fiber.Ctx) error {
	user := GetAuthUser(c)

	request, imagePath, err := h.parseCreatePostRequest(c)
	if err != nil {
		switch {
		case errors.Is(err, platformstorage.ErrImageTooLarge):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "image must be 10 MB or smaller",
			})
		case errors.Is(err, platformstorage.ErrUnsupportedImageType):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "image must be a jpeg, png, webp, heic, or heif file",
			})
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	shareLocation := true
	if request.ShareLocation != nil {
		shareLocation = *request.ShareLocation
	}

	post, err := h.postUseCase.CreatePost(ctx, user.ID, model.CreatePostInput{
		Kind:          firstNonEmpty(request.Category, request.Kind),
		Title:         request.Title,
		Body:          request.Body,
		LocationName:  request.LocationName,
		Latitude:      request.Latitude,
		Longitude:     request.Longitude,
		ShareLocation: shareLocation,
		Tags:          request.Tags,
		ImageURL:      request.ImageURL,
	})
	if err != nil {
		if imagePath != "" {
			_ = h.imageStore.Delete(imagePath)
		}
		return writePostError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(presentPostDetail(post))
}

func (h *PostHandler) parseCreatePostRequest(c *fiber.Ctx) (createPostRequest, string, error) {
	contentType := strings.ToLower(c.Get(fiber.HeaderContentType))
	if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
		return h.parseMultipartCreatePostRequest(c)
	}

	var request createPostRequest
	if err := c.BodyParser(&request); err != nil {
		return createPostRequest{}, "", errors.New("invalid request body")
	}

	return request, "", nil
}

func (h *PostHandler) parseMultipartCreatePostRequest(c *fiber.Ctx) (createPostRequest, string, error) {
	request := createPostRequest{
		Kind:         firstNonEmpty(c.FormValue("category"), c.FormValue("kind")),
		Title:        c.FormValue("title"),
		Body:         c.FormValue("body"),
		LocationName: c.FormValue("locationName"),
		ImageURL:     strings.TrimSpace(c.FormValue("imageUrl")),
	}
	if value := strings.TrimSpace(c.FormValue("shareLocation")); value != "" {
		parsed, parseErr := strconv.ParseBool(value)
		if parseErr != nil {
			return createPostRequest{}, "", errors.New("shareLocation is invalid")
		}
		request.ShareLocation = &parsed
	}

	latitude, err := strconv.ParseFloat(strings.TrimSpace(c.FormValue("latitude")), 64)
	if err != nil {
		return createPostRequest{}, "", errors.New("latitude is invalid")
	}
	longitude, err := strconv.ParseFloat(strings.TrimSpace(c.FormValue("longitude")), 64)
	if err != nil {
		return createPostRequest{}, "", errors.New("longitude is invalid")
	}
	request.Latitude = latitude
	request.Longitude = longitude

	if tagsValue := strings.TrimSpace(c.FormValue("tags")); tagsValue != "" {
		if strings.HasPrefix(tagsValue, "[") {
			if err := json.Unmarshal([]byte(tagsValue), &request.Tags); err != nil {
				return createPostRequest{}, "", errors.New("tags are invalid")
			}
		} else {
			request.Tags = strings.Split(tagsValue, ",")
		}
	}

	form, err := c.MultipartForm()
	if err != nil {
		return createPostRequest{}, "", errors.New("invalid form data")
	}

	files := form.File["image"]
	if len(files) == 0 || files[0] == nil {
		return request, "", nil
	}

	imagePath, saveErr := h.imageStore.Save(files[0])
	if saveErr != nil {
		return createPostRequest{}, "", saveErr
	}

	request.ImageURL = imagePath
	return request, imagePath, nil
}

func (h *PostHandler) toggleReaction(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	postID := c.Params("id")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	result, err := h.postUseCase.ToggleReaction(ctx, user.ID, postID)
	if err != nil {
		return writePostError(c, err)
	}

	return c.JSON(toggleReactionResponse{
		PostID:        result.PostID,
		ReactionCount: result.ReactionCount,
		IsReacted:     result.IsReacted,
	})
}

func (h *PostHandler) listComments(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	postID := c.Params("id")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	result, err := h.postUseCase.ListComments(ctx, user.ID, postID, model.ListPostCommentsInput{
		Cursor: c.Query("cursor"),
		Limit:  parseOptionalIntQuery(c, "limit"),
	})
	if err != nil {
		return writePostError(c, err)
	}

	items := make([]postCommentResponse, len(result.Items))
	for i, item := range result.Items {
		items[i] = presentPostComment(item)
	}

	return c.JSON(listCommentsResponse{
		Items:      items,
		NextCursor: result.NextCursor,
	})
}

func (h *PostHandler) createComment(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	postID := c.Params("id")

	var request createCommentRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	result, err := h.postUseCase.CreateComment(ctx, user.ID, postID, model.CreatePostCommentInput{
		ParentID: request.ParentID,
		Body:     request.Body,
	})
	if err != nil {
		return writePostError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(createCommentResponse{
		Comment:      presentPostComment(result.Comment),
		CommentCount: result.CommentCount,
	})
}

func (h *PostHandler) toggleCommentReaction(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	commentID := c.Params("commentId")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	reactionCount, isReacted, err := h.postUseCase.ToggleCommentReaction(ctx, user.ID, commentID)
	if err != nil {
		return writePostError(c, err)
	}

	return c.JSON(fiber.Map{
		"commentId":     commentID,
		"reactionCount": reactionCount,
		"isReacted":     isReacted,
	})
}

func (h *PostHandler) updateStatus(c *fiber.Ctx) error {
	user := GetAuthUser(c)
	postID := c.Params("id")

	var request updatePostStatusRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	post, err := h.postUseCase.UpdateStatus(ctx, user.ID, postID, model.UpdatePostStatusInput{
		Status:        request.Status,
		HelperUserIDs: request.HelperUserIDs,
	})
	if err != nil {
		return writePostError(c, err)
	}

	return c.JSON(presentPostDetail(post))
}

func (h *PostHandler) authorizeStreamUpgrade(c *fiber.Ctx) error {
	if !websocket.IsWebSocketUpgrade(c) {
		return fiber.ErrUpgradeRequired
	}

	accessToken := strings.TrimSpace(c.Query("accessToken"))
	if accessToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "access token is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := h.authUseCase.ValidateAccessToken(ctx, accessToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired access token",
		})
	}

	c.Locals(userContextKey, user)
	return c.Next()
}

func (h *PostHandler) stream(conn *websocket.Conn) {
	streamID, events := h.streamHub.Subscribe()
	defer h.streamHub.Unsubscribe(streamID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(25 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func presentPostSummary(post model.PostSummary) postSummaryResponse {
	tags := post.Tags
	if tags == nil {
		tags = []string{}
	}

	return postSummaryResponse{
		ID:      post.ID,
		Title:   post.Title,
		Excerpt: post.Excerpt,
		Kind:    post.Kind,
		Status:  post.Status,
		Author: postAuthorResponse{
			ID:   post.Author.ID,
			Name: post.Author.Name,
		},
		LocationName:     post.LocationName,
		Coordinates:      presentPostCoordinates(post.Coordinates),
		IsLocationShared: post.IsLocationShared,
		DistanceKm:       post.DistanceKm,
		ReactionCount:    post.ReactionCount,
		CommentCount:     post.CommentCount,
		IsReacted:        post.IsReacted,
		Tags:             tags,
		ImageURL:         post.ImageURL,
		CreatedAt:        post.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func presentPostCoordinates(coordinates *model.PostCoordinates) *postCoordinatesResponse {
	if coordinates == nil {
		return nil
	}

	return &postCoordinatesResponse{
		Latitude:  coordinates.Latitude,
		Longitude: coordinates.Longitude,
	}
}

func presentPostDetail(post model.PostDetail) postDetailResponse {
	return postDetailResponse{
		postSummaryResponse: presentPostSummary(post.PostSummary),
		Body:                post.Body,
		IsOwner:             post.IsOwner,
	}
}

func presentPostComment(comment model.PostComment) postCommentResponse {
	replies := make([]postCommentResponse, len(comment.Replies))
	for i, r := range comment.Replies {
		replies[i] = presentPostComment(r)
	}

	resp := postCommentResponse{
		ID:            comment.ID,
		ParentID:      comment.ParentID,
		Author:        postAuthorResponse{ID: comment.Author.ID, Name: comment.Author.Name},
		Body:          comment.Body,
		ReactionCount: comment.ReactionCount,
		IsReacted:     comment.IsReacted,
		ReplyCount:    comment.ReplyCount,
		CreatedAt:     comment.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     comment.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if len(replies) > 0 {
		resp.Replies = replies
	}
	return resp
}

func writePostError(c *fiber.Ctx, err error) error {
	var validationError model.ValidationError
	switch {
	case errors.As(err, &validationError):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": validationError.Error(),
		})
	case errors.Is(err, model.ErrPostNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	case errors.Is(err, model.ErrPostForbidden):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(),
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}
}

func parseOptionalFloatQuery(c *fiber.Ctx, key string) (*float64, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, errors.New("distanceKm must be a valid number")
	}

	return &value, nil
}

func parseOptionalIntQuery(c *fiber.Ctx, key string) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}

	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
