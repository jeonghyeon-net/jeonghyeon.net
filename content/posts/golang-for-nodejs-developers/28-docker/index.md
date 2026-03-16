# Docker

Go 바이너리는 외부 런타임 의존성이 없다. 이 특성이 Docker와 만나면 극단적으로 작은 이미지를 만들 수 있다. `node:alpine` 위에 `node_modules`를 올리는 대신, 빈 이미지 위에 바이너리 하나만 놓으면 된다.

## 전형적인 Node.js Dockerfile과의 차이

먼저 익숙한 Node.js Dockerfile을 보자:

```dockerfile
FROM node:22-alpine

WORKDIR /app
COPY package*.json ./
RUN npm ci --production
COPY . .

EXPOSE 3000
CMD ["node", "dist/index.js"]
```

이 이미지에는 Node.js 런타임, npm, libc, 셸, 각종 시스템 유틸리티가 포함된다. `node:22-alpine` 베이스만 약 180MB다. 여기에 `node_modules`가 추가되면 이미지 크기가 수백 MB에 이른다.

`.dockerignore`에서 `node_modules`를 제외하는 것도 필수다. 로컬의 `node_modules`가 컨테이너로 복사되면 플랫폼 불일치로 네이티브 모듈이 깨진다.

## Go의 멀티스테이지 빌드

Go는 멀티스테이지 빌드로 빌드 환경과 실행 환경을 분리한다:

```dockerfile
# 빌드 단계
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/server

# 실행 단계
FROM scratch
COPY --from=build /app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
```

두 단계로 나뉜다:

1. **빌드 단계** -- `golang:1.24` 이미지에서 소스를 컴파일한다. Go 컴파일러, 소스 코드, 의존성이 모두 이 단계에만 존재한다.
2. **실행 단계** -- `scratch`(완전히 빈 이미지)에 바이너리만 복사한다. 컴파일러도, 소스도, 셸도 없다.

최종 이미지에는 바이너리 하나만 들어간다. 크기는 보통 10~20MB 수준이다.

## CGO_ENABLED=0

`CGO_ENABLED=0`은 C 라이브러리 의존성을 제거하는 환경변수다. Go는 기본적으로 일부 표준 라이브러리(net, os/user 등)에서 시스템의 C 라이브러리를 사용한다. CGO가 활성화된 상태로 빌드하면 바이너리가 glibc나 musl에 동적 링크된다.

`scratch` 이미지에는 C 라이브러리가 없다. CGO를 비활성화하지 않으면 바이너리가 실행 시 다음과 같은 에러를 낸다:

```
standard_init_linux.go: exec user process caused "no such file or directory"
```

파일이 분명히 있는데 "no such file or directory"라는 혼란스러운 메시지다. 이는 바이너리 자체가 아니라 바이너리가 참조하는 동적 라이브러리(ld-linux.so)를 찾지 못한다는 뜻이다.

`CGO_ENABLED=0`으로 빌드하면 모든 코드가 순수 Go로 컴파일된다. 외부 라이브러리 의존성이 완전히 사라지므로 `scratch` 위에서 문제 없이 실행된다.

## scratch vs distroless vs alpine

실행 단계 베이스 이미지 선택지:

| 베이스 이미지 | 크기 | 셸 | 디버깅 도구 | TLS 인증서 |
|---|---|---|---|---|
| scratch | 0 MB | X | X | X |
| gcr.io/distroless/static | ~2 MB | X | X | O |
| alpine:3 | ~7 MB | O | O | O |

**scratch**는 말 그대로 아무것도 없다. TLS 인증서도 없으므로 HTTPS 요청을 보내는 애플리케이션이라면 인증서를 직접 복사해야 한다:

```dockerfile
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app /app
ENTRYPOINT ["/app"]
```

**distroless**는 Google이 관리하는 최소 이미지다. 셸이 없지만 TLS 인증서, 타임존 데이터 등 필수적인 런타임 파일을 포함한다. HTTPS 통신이 필요한 서비스라면 `scratch`보다 실용적이다.

**alpine**은 셸과 패키지 매니저(apk)가 있어 컨테이너에 접속해서 디버깅할 수 있다. 개발/스테이징 환경에 적합하다.

## 이미지 크기 비교

동일한 HTTP 서버를 Node.js와 Go로 빌드했을 때:

| 항목 | Node.js (node:22-alpine) | Go (scratch) |
|---|---|---|
| 베이스 이미지 | ~180 MB | 0 MB |
| 애플리케이션 코드 | ~50-200 MB (node_modules 포함) | ~10-15 MB (단일 바이너리) |
| 최종 이미지 | ~230-380 MB | ~10-15 MB |
| 컨테이너 시작 시간 | ~500ms-2s | ~10-50ms |

20배 이상의 크기 차이다. 이미지가 작으면 레지스트리 push/pull이 빠르고, 디스크 사용량이 줄고, 보안 공격 표면이 작아진다. `scratch` 이미지에는 셸도 없으므로 컨테이너에 침입하더라도 할 수 있는 것이 극히 제한된다.

## 실전 Dockerfile

27편에서 만든 Makefile의 ldflags를 Docker 빌드에 적용한다:

```dockerfile
FROM golang:1.24 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /app ./cmd/server

FROM gcr.io/distroless/static
COPY --from=build /app /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
```

몇 가지 포인트:

- `go.mod`와 `go.sum`을 먼저 복사하고 `go mod download`를 실행한다. Docker의 레이어 캐싱 덕분에 의존성이 변경되지 않으면 이 단계가 캐시된다. 소스 코드만 바뀌었을 때 의존성을 다시 받지 않는다.
- `ARG VERSION`으로 빌드 시 버전을 주입한다. `docker build --build-arg VERSION=1.2.3 .`으로 전달한다.
- `USER nonroot:nonroot`로 비루트 사용자로 실행한다. distroless 이미지에 이 사용자가 미리 정의되어 있다.

`package.json`을 먼저 복사하고 `npm ci`를 실행하는 레이어 캐싱 패턴과 동일하다. 다만 Go 모듈 캐시는 빌드 단계에만 존재하고 최종 이미지에 포함되지 않는다.

## docker compose로 DB와 함께 실행

로컬 개발 환경에서 애플리케이션과 데이터베이스를 함께 띄우는 구성:

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=db
      - DB_PORT=5432
      - DB_USER=app
      - DB_PASSWORD=secret
      - DB_NAME=myapp
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:17
    environment:
      - POSTGRES_USER=app
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=myapp
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  pgdata:
```

`depends_on`의 `condition: service_healthy`는 PostgreSQL이 실제로 연결을 받을 준비가 될 때까지 앱 시작을 지연한다. `depends_on`만 쓰면 컨테이너가 시작된 것만 확인하고, DB가 실제로 준비되었는지는 보장하지 않는다.

Go 이미지는 빌드 후 크기가 작으므로 `docker compose up --build`의 반복 사이클이 빠르다.

## health check

Docker는 컨테이너의 상태를 주기적으로 확인하는 health check를 지원한다. Dockerfile에 정의할 수도 있고 compose에서 정의할 수도 있다.

Go 애플리케이션에 health check endpoint를 추가한다:

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
})
```

compose에서 이 endpoint를 사용한다:

```yaml
services:
  app:
    build: .
    healthcheck:
      test: ["CMD", "/app", "-health"]
      interval: 10s
      timeout: 3s
      retries: 3
```

`scratch` 이미지에는 `curl`이나 `wget`이 없으므로 외부 도구로 HTTP 요청을 보낼 수 없다. 두 가지 해결 방법이 있다:

1. 바이너리 자체에 health check 모드를 구현한다. `-health` 플래그를 받으면 HTTP 요청을 보내고 결과를 반환하는 식이다.
2. distroless나 alpine 베이스를 사용한다.

첫 번째 방법의 구현:

```go
func main() {
    if len(os.Args) > 1 && os.Args[1] == "-health" {
        resp, err := http.Get("http://localhost:8080/healthz")
        if err != nil || resp.StatusCode != 200 {
            os.Exit(1)
        }
        os.Exit(0)
    }
    // 서버 시작 로직
}
```

같은 바이너리가 서버 모드와 health check 모드를 모두 처리한다. 추가 도구 설치 없이 `scratch` 이미지에서 동작한다.

## graceful shutdown과 Docker

Docker는 컨테이너를 중지할 때 SIGTERM 시그널을 보낸다. 22편에서 다룬 context를 활용하여 graceful shutdown을 구현한다:

```go
func main() {
    srv := &http.Server{Addr: ":8080"}

    go func() {
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatal(err)
    }
}
```

`signal.Notify`로 SIGTERM을 수신하면 `srv.Shutdown`을 호출한다. 이 메서드는 새로운 연결을 거부하면서 기존 연결이 완료될 때까지 기다린다. 10초 타임아웃 안에 완료되지 않으면 강제 종료된다.

Docker의 `stop_grace_period`(기본 10초)와 이 타임아웃을 맞춰야 한다. 애플리케이션의 shutdown 타임아웃이 Docker의 grace period보다 길면, Docker가 SIGKILL로 프로세스를 강제 종료한다.

Dockerfile에서 주의할 점이 하나 있다. 셸 형태의 CMD를 쓰면 SIGTERM이 Go 프로세스에 전달되지 않는다:

```dockerfile
# 셸 형태 -- SIGTERM이 셸에 전달된다
CMD /app

# exec 형태 -- SIGTERM이 Go 프로세스에 직접 전달된다
ENTRYPOINT ["/app"]
```

셸 형태는 `/bin/sh -c /app`으로 실행되어 Go 프로세스가 셸의 자식 프로세스가 된다. `scratch` 이미지에는 셸이 없으므로 자연스럽게 exec 형태만 사용하게 된다.

## 빌드 시간 최적화

Docker 빌드 캐시를 최대한 활용하는 Dockerfile 구조:

```dockerfile
FROM golang:1.24 AS build
WORKDIR /src

# 1. 의존성 레이어 (go.mod/go.sum 변경 시만 재실행)
COPY go.mod go.sum ./
RUN go mod download

# 2. 소스 레이어 (코드 변경 시 재실행)
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/server

FROM gcr.io/distroless/static
COPY --from=build /app /app
ENTRYPOINT ["/app"]
```

레이어 순서가 중요하다. 자주 변경되는 내용일수록 아래에 배치한다. `go.mod`는 의존성을 추가할 때만 바뀌고, 소스 코드는 매 커밋마다 바뀐다. 이 순서를 지키면 소스만 변경했을 때 `go mod download` 레이어가 캐시에서 재사용된다.

Go의 빌드 캐시도 Docker 레이어로 마운트하면 반복 빌드가 빨라진다:

```dockerfile
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/server
```

`--mount=type=cache`는 Docker BuildKit의 캐시 마운트다. Go 컴파일러의 빌드 캐시(`/root/.cache/go-build`)와 모듈 캐시(`/go/pkg/mod`)를 빌드 간에 공유한다. 파일 하나를 고치고 다시 빌드하면 변경된 패키지만 재컴파일한다.

Go의 Docker 이미지는 작고 빠르다. 외부 런타임 의존성이 없으므로 `scratch` 위에 바이너리 하나만 놓으면 프로덕션 배포가 가능하다. 서버리스 환경이나 Kubernetes에서 pod이 빠르게 스케일 아웃해야 할 때 이 차이가 실질적으로 체감된다.

---

28편에 걸쳐 Node.js 개발자의 관점에서 Go를 살펴봤다. 두 언어는 철학이 다르고 잘하는 영역도 다르다. 이 시리즈가 Go를 시작하는 데 필요한 맥락을 제공했길 바란다.
