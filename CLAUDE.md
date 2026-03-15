# jeonghyeon.net

정적 사이트 생성기. 마크다운 → HTML.

## 콘텐츠 규칙 (위반 시 빌드 실패)

- HTML 태그 금지, frontmatter 금지
- 모든 `.md`는 H1(`#`)으로 시작 (`_layout/` 예외)
- 이미지: `.webp`만 허용, 파일명 공백 금지
- 제목은 H1에서, 설명은 첫 문단에서 자동 추출. 메타데이터 없음

## 파일 구조

```
content/posts/글-slug/index.md                    # 단독 글
content/posts/시리즈-slug/01-제목/index.md         # 시리즈 (2자리 번호 필수)
content/페이지-slug/index.md                       # 일반 페이지
```

시리즈: 번호 있는 폴더와 없는 폴더 혼용 금지, 번호 중복 금지.
하위 폴더가 있는 디렉토리의 `index.md`는 pre-commit hook이 자동 생성하므로 수정 금지.

## 이미지

이미지 참조: `![설명](파일명.webp)`. `make optimize`로 WebP 변환 + 리사이즈.

## 빌드

- `make build` — 전체 빌드 → `dist/`
- `make serve` — 로컬 서버 + 자동 재빌드
- `make optimize` — 이미지 WebP 변환
- 에디터: `cd editor && pnpm tauri dev`

## 페르소나

- 한국어, 평서체(~다 체), 1인칭 금지, 이모지 금지
- 기술 용어는 영어 원문 그대로 (예: "rendering")
- 간결하게. 불필요한 수식어 없이 핵심만
- 코드 예제는 실행 가능한 최소 단위
