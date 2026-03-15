# jeonghyeon.net

개발자이자 음악가인 jeonghyeon의 개인 웹사이트. 순수 HTML만 출력하는 정적 사이트 생성기.

## 콘텐츠 작성 규칙

**절대 위반 불가 — 위반 시 빌드 실패:**

- 마크다운 안에 HTML 태그 금지 (`<div>`, `<br>`, `<img>` 등 전부)
- frontmatter 금지 (파일 첫 줄이 `---`이면 안 됨)
- 모든 `.md` 파일은 반드시 H1(`#`)으로 시작해야 함 (`_layout/` 파일은 예외)
- 이미지는 `.webp`만 허용
- 이미지 파일명에 공백 금지 (마크다운 이미지 링크가 깨짐)
- 블로그 글은 반드시 `content/posts/글-이름/index.md` 구조

**마크다운만 쓴다.** 제목은 H1에서, 설명은 첫 번째 문단에서 자동 추출된다. 별도 설정 파일이나 메타데이터는 없다.

## 글 작성 방법

### 단독 블로그 글

```
content/posts/글-slug/
  index.md       # 본문
  diagram.webp   # 이미지 (선택)
```

마크다운에서 이미지 참조: `![설명](diagram.webp)`

### 시리즈 (연재물)

```
content/posts/시리즈-slug/
  01-첫번째/
    index.md
  02-두번째/
    index.md
    screenshot.webp
```

- 번호는 반드시 2자리 (`01`~`99`)
- 번호 있는 폴더와 없는 폴더 혼용 금지
- 번호 중복 금지, 비연속은 허용

### 일반 페이지 (블로그 아닌 것)

```
content/페이지-slug/
  index.md
```

루트 레벨 예외: `content/index.md`, `content/404.md`

## 블로그 목록/시리즈 목록

`index.md`는 pre-commit hook이 자동으로 생성/갱신한다. 하위 폴더가 있는 디렉토리의 `index.md`는 항상 자동 생성으로 덮어쓰므로 직접 수정하지 않는다.

## 이미지 추가 워크플로우

1. 아무 포맷(PNG, JPG 등)으로 이미지를 추가
2. `make optimize` 실행 — WebP로 변환 + 리사이즈(최대 700px) + 마크다운 참조 자동 업데이트 + 원본 삭제
3. 커밋

에디터 앱(`editor/`)에서는 이미지를 에디터에 드래그앤드롭하면 자동으로 WebP 변환 + 마크다운 삽입된다.

## 빌드/배포

- `make lint` — 정책 검증
- `make build` — HTML 생성 + 최소화 → `dist/`
- `make serve` — 빌드 + wrangler 로컬 서버 + 파일 변경 자동 재빌드
- `make optimize` — 이미지 WebP 변환 + 리사이즈 (최대 700px)
- `make clean` — `dist/` 삭제
- `make hooks` — git hooks 설치 (최초 1회)
- main에 push하면 GitHub Actions가 Cloudflare Pages에 자동 배포

### Transformer 커맨드

```
transformer lint <content-dir>           # 정책 검증
transformer index <content-dir>          # 목록 index.md 자동 생성/갱신
transformer render <content-dir> <dist>  # 전체 렌더링
transformer render-single <content-dir> <md-path>  # 단일 파일 렌더링 (에디터 미리보기용)
transformer minify <dist-dir>            # HTML 최소화
transformer check <dist-dir>             # CSS/JS 오염 검사
transformer build <content-dir> <dist>   # lint+index+render+minify+check 전체
transformer watch <content-dir> <dist>   # 파일 변경 감시 + 자동 재빌드
```

## 디렉터리 구조

```
content/           # 콘텐츠 (마크다운 + 이미지 + robots.txt)
  _layout/         # 헤더/푸터 (페이지 아님)
  index.yaml       # 사이트 설정 (이름, URL, 작성자, 폰트, 너비, 섹션 제목)
transformer/       # Go 빌드 도구
hooks/             # git hooks (pre-commit: lint+index, pre-push: WebP 검사)
editor/            # Tauri 데스크톱 에디터 앱 (React + Rust)
dist/              # 빌드 결과물 (gitignore)
```

### 에디터 앱 (`editor/`)

jeonghyeon.net 콘텐츠 관리용 Tauri v2 데스크톱 앱.

- **실행**: `cd editor && pnpm tauri dev`
- **빌드**: `cd editor && pnpm tauri build`
- 마크다운 에디터 (CodeMirror) + 실시간 HTML 미리보기 (iframe)
- xterm.js 터미널 (한국어 IME, 다중 세션)
- 파일트리 (우클릭: New Post, Rename, Delete)
- 이미지 드래그앤드롭 → WebP 자동 변환 + 마크다운 삽입
- Win98 레트로 UI 테마
- dev 모드: 프로젝트 디렉토리 직접 사용
- production: `~/Library/Application Support/net.jeonghyeon.editor/repo/`에 SSH 클론

## 페르소나

이 사이트의 글은 다음 페르소나로 작성한다:

- 개발자이자 음악가. 두 분야를 넘나드는 시각을 가지고 있다.
- 기술 글은 깊이 있되 간결하게. 불필요한 수식어나 과장 없이 핵심만.
- 한국어로 작성. 기술 용어는 영어 원문 그대로 쓴다 (예: "렌더링"이 아닌 "rendering").
- 1인칭은 쓰지 않는다. "저는", "나는" 대신 비인칭 서술.
- 이모지 사용하지 않는다.
- 말투는 평서체 (해요체/합니다체 아닌 ~다 체).
- 코드 예제는 실행 가능한 최소 단위로.
- 음악 관련 글은 기술적 분석과 감상을 병행할 수 있다.
