# jeonghyeon.net

개발자이자 음악가인 jeonghyeon의 개인 웹사이트. 순수 HTML만 출력하는 정적 사이트 생성기.

## 콘텐츠 작성 규칙

**절대 위반 불가 — 위반 시 빌드 실패:**

- 마크다운 안에 HTML 태그 금지 (`<div>`, `<br>`, `<img>` 등 전부)
- frontmatter 금지 (파일 첫 줄이 `---`이면 안 됨)
- 모든 `.md` 파일은 반드시 H1(`#`)으로 시작해야 함 (`_layout/` 파일은 예외)
- 이미지는 `.webp`만 허용
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

`index.md`가 없는 폴더는 pre-commit hook이 자동으로 목록 마크다운을 생성한다. 직접 `index.md`를 작성하면 자동 생성을 덮어쓴다.

## 이미지 추가 워크플로우

1. 아무 포맷(PNG, JPG 등)으로 이미지를 추가
2. `make optimize` 실행 — WebP로 변환 + 마크다운 참조 자동 업데이트 + 원본 삭제
3. 커밋

## 빌드/배포

- `make lint` — 정책 검증
- `make build` — HTML 생성 + 최소화 → `dist/`
- `make serve` — 빌드 + wrangler 로컬 서버 + 파일 변경 자동 재빌드
- `make optimize` — 이미지 WebP 변환 + 리사이즈 (최대 700px)
- `make clean` — `dist/` 삭제
- `make hooks` — git hooks 설치 (최초 1회)
- main에 push하면 GitHub Actions가 Cloudflare Pages에 자동 배포

## 디렉터리 구조

```
content/           # 콘텐츠 (마크다운 + 이미지 + robots.txt)
  _layout/         # 헤더/푸터 (페이지 아님)
  badges/          # 88x31 배지 이미지 (footer에서 절대경로로 참조)
transformer/       # Go 빌드 도구
hooks/             # git hooks (pre-commit, pre-push)
dist/              # 빌드 결과물 (gitignore)
```

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
