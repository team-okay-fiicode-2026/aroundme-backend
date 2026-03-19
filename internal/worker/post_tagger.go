package worker

import (
	"context"
	"log"
	"time"

	"github.com/aroundme/aroundme-backend/internal/platform/ai"
	"github.com/aroundme/aroundme-backend/internal/repository"
	"github.com/aroundme/aroundme-backend/internal/usecase"
)

const (
	taggerPollInterval = 30 * time.Second
	taggerBatchSize    = 20
)

// PostTaggerWorker polls for untagged posts, enriches their tags via Claude,
// then triggers skill-match notifications for matched users.
type PostTaggerWorker struct {
	postRepo repository.PostRepository
	notifier usecase.PostNotifier
	tagger   ai.Tagger
}

func NewPostTaggerWorker(
	postRepo repository.PostRepository,
	notifier usecase.PostNotifier,
	tagger ai.Tagger,
) *PostTaggerWorker {
	return &PostTaggerWorker{
		postRepo: postRepo,
		notifier: notifier,
		tagger:   tagger,
	}
}

// Run starts the polling loop. It blocks until ctx is cancelled.
func (w *PostTaggerWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(taggerPollInterval)
	defer ticker.Stop()

	// Process immediately on startup, then on each tick.
	w.processBatch(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *PostTaggerWorker) processBatch(ctx context.Context) {
	posts, err := w.postRepo.ClaimUntaggedPosts(ctx, taggerBatchSize)
	if err != nil {
		log.Printf("post_tagger: claim untagged posts: %v", err)
		return
	}
	if len(posts) == 0 {
		return
	}

	for _, post := range posts {
		aiTags, err := w.tagger.ExtractTags(ctx, post.Title, post.Body)
		if err != nil {
			log.Printf("post_tagger: extract tags for post %s: %v — using existing tags", post.ID, err)
			// Notifications still fire with the rule-based tags already on the post.
			go w.notifier.NotifySkillMatchPost(context.Background(), post)
			continue
		}

		merged := mergeTags(post.Tags, aiTags)
		if err := w.postRepo.UpdateAITags(ctx, post.ID, merged); err != nil {
			log.Printf("post_tagger: update tags for post %s: %v", post.ID, err)
		}

		post.Tags = merged
		go w.notifier.NotifySkillMatchPost(context.Background(), post)
	}
}

// mergeTags combines existing (rule-based) tags with AI-generated tags, deduplicating.
func mergeTags(existing, aiTags []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(aiTags))
	result := make([]string, 0, len(existing)+len(aiTags))
	for _, t := range existing {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	for _, t := range aiTags {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	return result
}
