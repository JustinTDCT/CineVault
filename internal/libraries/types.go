package libraries

type LibraryType string

const (
	TypeMovies      LibraryType = "movies"
	TypeTVShows     LibraryType = "tv_shows"
	TypeAdultMovies LibraryType = "adult_movies"
	TypeAdultClips  LibraryType = "adult_clips"
	TypeHomeMovies  LibraryType = "home_movies"
	TypeOtherMovies LibraryType = "other_movies"
	TypeMusic       LibraryType = "music"
	TypeMusicVideos LibraryType = "music_videos"
	TypeAudiobooks  LibraryType = "audiobooks"
	TypeEbooks      LibraryType = "ebooks"
	TypeComicBooks  LibraryType = "comic_books"
)

var AllTypes = []LibraryType{
	TypeMovies, TypeTVShows, TypeAdultMovies, TypeAdultClips,
	TypeHomeMovies, TypeOtherMovies, TypeMusic, TypeMusicVideos,
	TypeAudiobooks, TypeEbooks, TypeComicBooks,
}

func (t LibraryType) CacheServerType() string {
	switch t {
	case TypeMovies:
		return "movie"
	case TypeTVShows:
		return "tv_show"
	case TypeAdultMovies, TypeAdultClips:
		return "adult_movie"
	case TypeHomeMovies:
		return "home_video"
	case TypeMusic, TypeMusicVideos:
		return "music_track"
	case TypeAudiobooks:
		return "audiobook"
	case TypeEbooks:
		return "ebook"
	case TypeComicBooks:
		return "comic_book"
	default:
		return ""
	}
}

func (t LibraryType) HasMetadata() bool {
	return t != TypeOtherMovies && t != TypeHomeMovies
}

func (t LibraryType) IsVideo() bool {
	switch t {
	case TypeMovies, TypeTVShows, TypeAdultMovies, TypeAdultClips,
		TypeHomeMovies, TypeOtherMovies, TypeMusicVideos:
		return true
	}
	return false
}

func (t LibraryType) HasAudio() bool {
	switch t {
	case TypeMusic, TypeMusicVideos, TypeAudiobooks,
		TypeMovies, TypeTVShows, TypeAdultMovies, TypeAdultClips,
		TypeHomeMovies, TypeOtherMovies:
		return true
	}
	return false
}

func (t LibraryType) Label() string {
	labels := map[LibraryType]string{
		TypeMovies:      "Movies",
		TypeTVShows:     "TV Shows",
		TypeAdultMovies: "Adult Movies",
		TypeAdultClips:  "Adult Clips",
		TypeHomeMovies:  "Home Movies",
		TypeOtherMovies: "Other Movies",
		TypeMusic:       "Music",
		TypeMusicVideos: "Music Videos",
		TypeAudiobooks:  "Audiobooks",
		TypeEbooks:      "eBooks",
		TypeComicBooks:  "Comic Books",
	}
	if l, ok := labels[t]; ok {
		return l
	}
	return string(t)
}

func (t LibraryType) Valid() bool {
	for _, lt := range AllTypes {
		if lt == t {
			return true
		}
	}
	return false
}
