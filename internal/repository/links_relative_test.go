package repository

import "testing"

func TestResolveAssetLinkPathCanonicalization(t *testing.T) {
	for _, tc := range []struct {
		href, want string
		valid      bool
	}{
		{"./guide.md#top", "guide.md", true},
		{"nested/../guide.md", "guide.md", true},
		{"nested/./guide.md", "nested/guide.md", true},
		{"nested/../../guide.md", "", false},
		{".", "", false},
		{"./", "", false},
		{"../guide.md", "", false},
		{"/guide.md", "", false},
		{"#part", "", false},
		{"mailto:user@example.org", "", false},
	} {
		got, valid := ResolveAssetLinkPath(tc.href)
		if got != tc.want || valid != tc.valid {
			t.Fatal(tc, got, valid)
		}
	}
}

func TestRelativeLinkRenameAndRebaseCoverage(t *testing.T) {
	t.Run("relative helper", func(t *testing.T) {
		for _, tc := range []struct {
			fromDir, target, want string
		}{
			{"", "/guide.md", "guide.md"},
			{"folder", "/folder/guide.md", "guide.md"},
			{"folder/sub", "/folder/guide.md", "../guide.md"},
			{"folder", "/other/guide.md", "../other/guide.md"},
		} {
			if got := relativeLinkPath(tc.fromDir, tc.target); got != tc.want {
				t.Fatal(tc, got)
			}
		}
	})

	t.Run("rename", func(t *testing.T) {
		for _, tc := range []struct {
			content, source, oldPath, newPath, want string
			changed                                 bool
		}{
			{
				content: "[doc](./docs/../target.md#part)",
				source:  "/guides/source.md",
				oldPath: "/guides/target.md",
				newPath: "/manual/renamed.md",
				want:    "[doc](../manual/renamed.md#part)",
				changed: true,
			},
			{
				content: "[root](child/guide.md) [external](https://example.org) [absolute](/fixed.md) [self](#part)",
				source:  "/source.md",
				oldPath: "/child/guide.md",
				newPath: "/docs/guide.md",
				want:    "[root](docs/guide.md) [external](https://example.org) [absolute](/fixed.md) [self](#part)",
				changed: true,
			},
			{
				content: "[other](child/guide.md)",
				source:  "/source.md",
				oldPath: "/elsewhere.md",
				newPath: "/docs/guide.md",
				want:    "[other](child/guide.md)",
				changed: false,
			},
		} {
			got := RewriteLinksForRenameFrom(tc.content, tc.source, tc.oldPath, tc.newPath)
			if got.Content != tc.want || got.Changed != tc.changed {
				t.Fatal(tc, got)
			}
		}
	})

	t.Run("rebase", func(t *testing.T) {
		for _, tc := range []struct {
			content, oldSource, newSource, want string
			changed                             bool
		}{
			{
				content:   "[same](./target.md#part) [nested](docs/../asset.png) [up](../root.md) [absolute](/fixed.md) [self](#part) [external](https://example.org)",
				oldSource: "/guides/source.md",
				newSource: "/docs/source.md",
				want:      "[same](../guides/target.md#part) [nested](../guides/asset.png) [up](../root.md) [absolute](/fixed.md) [self](#part) [external](https://example.org)",
				changed:   true,
			},
			{
				content:   "[absolute](/fixed.md) [self](#part) [external](https://example.org)",
				oldSource: "/guides/source.md",
				newSource: "/docs/source.md",
				want:      "[absolute](/fixed.md) [self](#part) [external](https://example.org)",
				changed:   false,
			},
		} {
			got := RebaseRelativeLinks(tc.content, tc.oldSource, tc.newSource)
			if got.Content != tc.want || got.Changed != tc.changed {
				t.Fatal(tc, got)
			}
		}
	})
}

func TestRelativeLinkRootTargets(t *testing.T) {
	for _, tc := range []struct{ from, target, want string }{{"/a", "/", ".."}, {"/a", "/a", "."}, {"/", "/", "."}} {
		if got := relativeLinkPath(tc.from, tc.target); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestRelativeRewriteOverlappingDestinations(t *testing.T) {
	input := "[a](longtarget.md) [b](longtarget.md#section) [c](longtarget.md#section) `longtarget.md`"
	want := "[a](x.md) [b](x.md#section) [c](x.md#section) `longtarget.md`"
	for range 50 {
		got := RewriteLinksForRenameFrom(input, "/source.md", "/longtarget.md", "/x.md")
		if got.Content != want || !got.Changed {
			t.Fatal(got)
		}
	}
}

func TestRelativeRenamePreservesAnchorSource(t *testing.T) {
	input := "[x](target.md#foo&amp;bar) [y](<target.md#space here>)"
	got := RewriteLinksForRenameFrom(input, "/folder/source.md", "/folder/target.md", "/folder/next.md")
	if got.Content != "[x](next.md#foo&amp;bar) [y](<next.md#space here>)" {
		t.Fatal(got)
	}
}
