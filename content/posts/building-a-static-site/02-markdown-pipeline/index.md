# Markdown 변환 파이프라인

마크다운 파일이 HTML이 되기까지의 과정을 정리한다.

## 로컬 파이프라인

커밋할 때 pre-commit hook이 두 가지를 실행한다.

1. **content-linter** — 마크다운 정책을 검증한다. HTML 태그가 있으면 거부하고, frontmatter가 있으면 거부한다. 순수 마크다운만 통과시킨다.
2. **auto-index** — 목록 페이지가 없는 폴더에 자동으로 인덱스 마크다운을 생성한다. 시리즈 폴더는 순서 목록으로, 일반 폴더는 알파벳순으로.

## CI 파이프라인

push하면 GitHub Actions가 나머지를 처리한다.

1. **md-to-html** — goldmark으로 마크다운을 parsing하고 HTML로 변환한다. 헤더와 푸터를 감싸고, meta 태그를 삽입한다.
2. **html-minifier** — tdewolff/minify로 공백을 제거한다.
3. **배포** — wrangler로 Cloudflare Pages에 올린다.

## 왜 Go인가

Go는 single binary로 컴파일된다. CI에서 `go build` 한 번이면 끝이다. Node.js처럼 의존성을 설치할 필요가 없다.
