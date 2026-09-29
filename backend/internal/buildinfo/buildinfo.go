package buildinfo

// Values are replaced with -ldflags in production images. Development builds
// intentionally remain explicit instead of pretending to be a release.
var (
	Version   = "dev"
	CommitSHA = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commitSha"`
	BuildTime string `json:"buildTime"`
}

func Current() Info {
	return Info{Version: Version, CommitSHA: CommitSHA, BuildTime: BuildTime}
}
