# 배포 자동화

코드를 push하면 사이트가 업데이트된다. 그 사이에 수동 작업은 없다.

## GitHub Actions

workflow는 단순하다. content/, transformer/, Makefile이 변경되면 트리거된다.

```yaml
on:
  push:
    branches: [main]
    paths:
      - 'content/**'
      - 'transformer/**'
      - 'Makefile'
```

Go를 설치하고, `make build`를 실행하고, wrangler로 배포한다. 3단계.

## Cloudflare Pages

정적 파일 호스팅으로 Cloudflare Pages를 선택한 이유는 간단하다. 무료이고, 빠르고, 커스텀 도메인 설정이 쉽다. `dist/` 폴더를 통째로 올리면 된다.

## 결과

글을 쓰고, 커밋하고, push한다. 1분 안에 사이트에 반영된다.
