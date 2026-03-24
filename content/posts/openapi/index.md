# OpenAPI Spec으로 Next.js 프론트엔드 코드젠하기

백엔드에서 OpenAPI spec 파일을 넘겨줬음. 이제 프론트엔드에서 API를 호출해야 함. 여기서 갈림길이 나옴.

하나는 spec을 눈으로 읽으면서 타입을 하나하나 만들고, fetch 함수를 직접 짜는 거임. 다른 하나는 spec에서 타입, client, hook, mock을 자동 생성하는 거임. 전자는 spec이 바뀔 때마다 사람이 수동으로 반영해야 하고, 빠뜨리면 런타임에 터짐. 후자는 코드젠 한 번 돌리면 끝이고, 빠뜨리면 컴파일러가 알려줌.

이건 소규모 사이드 프로젝트 얘기가 아님. Stripe의 개발자 플랫폼 팀에 있던 Alex Rattray라는 엔지니어가 나와서 Stainless라는 회사를 만들었는데, 이 회사가 OpenAI, Anthropic, Cloudflare의 공식 SDK를 OpenAPI spec 기반으로 생성하고 있음. GitHub의 Octokit SDK도 OpenAPI spec에서 자동 생성됨. "spec에서 코드를 뽑는다"는 접근이 업계 표준이 된 지 오래임.

문제는 도구가 너무 많다는 거임.

## 뭐가 있는지부터 보자

도구마다 "어디까지 생성해주느냐"가 다름. 타입만 뽑는 것부터 hook, mock, validation까지 전부 뱉는 것까지 스펙트럼이 넓음. 아래는 2025년 기준 수치임.

| 도구 | npm 주간 다운로드 | 뭘 생성하나 |
|---|---|---|
| openapi-typescript | ~2.1M | `.d.ts` 타입 파일만. 런타임 코드 0 |
| @hey-api/openapi-ts | ~977K | SDK 함수, 타입, Zod, TanStack Query hook |
| openapi-generator | ~1.1M | 40개+ 언어 client. Java 기반 원조 |
| orval | ~772K | hook, MSW mock, Zod, 타입 |
| kubb | ~72K | 전부. 플러그인으로 조합 |
| RTK Query codegen | ~129K | RTK Query API slice |

다운로드 수가 곧 품질은 아님. openapi-generator가 1.1M이지만 TypeScript 생성 품질은 하위권임. 각 도구가 풀고 있는 문제가 다르기 때문에, 숫자보다는 자기 상황에 맞는 걸 고르는 게 맞음.

이 글에서는 TypeScript + Next.js 환경에서 실제로 쓸 만한 도구 다섯 개를 다룸. openapi-generator는 polyglot 환경(하나의 spec에서 Go 서버 + TS client + Python SDK를 동시에 뽑아야 하는 경우) 전용이라 여기서는 빼겠음.

## 타입만 뽑아서 직접 조립하기: openapi-typescript + openapi-fetch

가장 인기 있는 조합이고, 철학이 뚜렷함. "런타임 코드는 생성하지 않는다." spec에서 `.d.ts` 타입 파일만 뽑고, 실제 API 호출은 openapi-fetch라는 ~6kb짜리 fetch 래퍼가 그 타입을 활용해서 type-safe하게 처리함.

```bash
npm install openapi-fetch
npm install -D openapi-typescript
```

```bash
npx openapi-typescript ./spec.json -o ./src/api/schema.d.ts
```

이렇게 하면 spec의 모든 endpoint 정보가 `paths`라는 타입에 담김. 이걸 client에 넘기면 끝임:

```typescript
// src/api/client.ts
import createClient from "openapi-fetch";
import type { paths } from "./schema";

export const api = createClient<paths>({
  baseUrl: process.env.NEXT_PUBLIC_API_URL,
});
```

호출할 때 path, query, body, response 타입이 전부 자동 추론됨:

```typescript
const { data, error } = await api.GET("/users/{id}", {
  params: {
    path: { id: "123" },
    query: { include: "profile" },
  },
});
// data 타입이 spec에서 추론됨. 잘못된 필드 넣으면 컴파일 에러.
// error 타입도 spec에 정의된 에러 응답(4xx, 5xx) 기반으로 추론됨.
```

Server Component에서 바로 쓸 수 있고, fetch 기반이라 Next.js의 캐싱과 request deduplication이 그대로 적용됨:

```typescript
// app/users/[id]/page.tsx
import { api } from "@/api/client";

export default async function UserPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const { data } = await api.GET("/users/{id}", {
    params: { path: { id } },
  });

  if (!data) return <div>Not Found</div>;
  return <h1>{data.name}</h1>;
}
```

여기까지는 완벽해 보이는데, Client Component에서 React Query를 쓰고 싶으면 얘기가 달라짐:

```typescript
"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";

export function UserProfile({ id }: { id: string }) {
  const { data } = useQuery({
    queryKey: ["user", id],
    queryFn: () =>
      api.GET("/users/{id}", {
        params: { path: { id } },
      }),
    select: (res) => res.data,
  });

  return <div>{data?.name}</div>;
}
```

queryKey를 직접 만들고, queryFn을 직접 연결하고, select로 response를 까줘야 함. endpoint가 5개일 때는 괜찮은데, 50개를 넘어가면 이 boilerplate가 고통이 됨. "hook을 자동으로 생성해주는 도구는 없나?"라는 생각이 드는 시점이 옴. 있음. 근데 그 전에, 위 코드에서 쓴 React Query가 뭔지부터 짚고 가겠음.

## 잠깐 — data fetching 라이브러리 이야기

코드젠 도구들이 "hook을 생성한다"고 할 때, 그 hook은 결국 어떤 data fetching 라이브러리의 hook임. 대표적인 선택지가 세 가지 있음.

**TanStack Query (React Query)**: 가장 널리 쓰이는 server state 관리 라이브러리. API 응답을 캐싱하고, 백그라운드에서 자동 refetch하고, 낙관적 업데이트를 지원함. `useQuery`로 데이터를 가져오고 `useMutation`으로 변경 요청을 보내는 패턴임. 위에서 본 queryKey, queryFn, select가 이 라이브러리의 API임. React뿐 아니라 Vue, Svelte, Solid, Angular 버전도 있어서 TanStack Query라는 이름으로 통합됨.

**SWR**: Vercel이 만든 data fetching 라이브러리. TanStack Query보다 API가 단순함. "stale-while-revalidate" 전략이 이름의 유래인데, 캐시된 데이터를 먼저 보여주고 백그라운드에서 최신 데이터를 가져오는 방식임. 설정이 적고 가볍지만, mutation 처리나 캐시 무효화 같은 고급 기능은 TanStack Query가 더 풍부함.

**RTK Query**: Redux Toolkit에 내장된 data fetching 솔루션. 별도의 라이브러리를 추가하지 않고 Redux 생태계 안에서 server state를 관리할 수 있음. Redux DevTools로 API 캐시 상태를 직접 확인할 수 있다는 고유한 장점이 있음. 다만 Redux Toolkit을 안 쓰고 있는 프로젝트에서 이것만을 위해 Redux를 도입하는 건 과함.

이 세 라이브러리는 전부 "server state"를 다루는 도구임. 버튼 클릭 여부나 모달 열림/닫힘 같은 "client state"와는 관심사가 다름. client state는 `useState`나 Zustand, Jotai 같은 도구가 담당하고, server state는 위 라이브러리들이 담당함.

코드젠과의 관계는 단순함. 이 라이브러리들의 hook을 직접 짜는 대신, 코드젠 도구가 spec에서 자동으로 생성해주는 거임. 어떤 라이브러리의 hook을 생성하느냐에 따라 코드젠 도구 선택이 달라짐:

- **TanStack Query**: @hey-api/openapi-ts, orval, kubb 전부 지원
- **SWR**: orval, kubb 지원
- **RTK Query**: RTK Query codegen 전용

TanStack Query가 가장 선택지가 넓음. 특별한 이유가 없으면 TanStack Query 기반으로 가는 게 코드젠 도구 선택의 폭을 가장 넓게 유지하는 방법임.

## hook까지 다 생성하기: 세 갈래

### @hey-api/openapi-ts — 올인원 SDK 생성기

이름이 좀 낯선데, 원래 `openapi-typescript-codegen`이라는 꽤 유명한 프로젝트였음. 원작자(ferdikoomen)가 관리를 멈추면서 2024년 5월에 공식 archived됐고, 커뮤니티가 fork해서 `@hey-api/openapi-ts`라는 이름으로 이어받은 거임. npm 주간 977K, GitHub에 Vercel과 PayPal이 사용한다고 명시되어 있을 만큼 주류 도구임.

원본과 달리 플러그인 아키텍처로 재설계됨. 타입, SDK 함수, TanStack Query hook, Zod schema를 플러그인 단위로 골라서 생성할 수 있음:

```typescript
// openapi-ts.config.ts
import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "./spec.json",
  output: { path: "./src/api/generated" },
  plugins: [
    "@hey-api/typescript",
    "@hey-api/sdk",
    {
      name: "@tanstack/react-query",
      queryOptions: true,
    },
  ],
});
```

```bash
npx @hey-api/openapi-ts
```

아까 openapi-fetch에서 직접 짰던 boilerplate가 사라짐:

```typescript
"use client";

import { useQuery } from "@tanstack/react-query";
import { getUserOptions } from "@/api/generated";

export function UserProfile({ id }: { id: string }) {
  const { data } = useQuery({
    ...getUserOptions({ path: { id } }),
  });
  return <div>{data?.name}</div>;
}
```

endpoint마다 `getUserOptions`, `createUserMutation` 같은 option factory가 자동 생성되고, queryKey 관리도 알아서 됨. Zod schema가 필요하면 플러그인 하나(`@hey-api/zod`) 추가하면 되고, API response를 런타임에 검증할 수 있음. 외부 API를 호출하는 경우 spec과 실제 응답이 다를 수 있는데, 그때 유용함.

### orval — mock이 진짜 강점

orval도 React Query hook을 생성하는데, 이 도구의 킬러 피처는 따로 있음. MSW mock handler와 Faker.js 기반 더미 데이터를 같이 생성한다는 거임.

```typescript
// orval.config.ts
import { defineConfig } from "orval";

export default defineConfig({
  api: {
    input: {
      target: "./spec.json",
    },
    output: {
      target: "./src/api/generated.ts",
      client: "react-query",
      mock: true,
    },
  },
});
```

```bash
npx orval
```

hook 사용은 직관적임:

```typescript
"use client";

import { useGetUser } from "@/api/generated";

export function UserProfile({ id }: { id: string }) {
  const { data } = useGetUser(id);
  return <div>{data?.name}</div>;
}
```

`mock: true`로 설정해뒀으니 MSW handler도 같이 생성됨:

```typescript
import { getGetUserMockHandler } from "@/api/generated";
import { setupWorker } from "msw/browser";

const worker = setupWorker(getGetUserMockHandler());
worker.start();
```

spec에 `name`이 string으로 정의되어 있으면 Faker.js가 사람 이름을 넣어주고, `email`이면 이메일 형식을 넣어줌. 백엔드 API가 아직 안 만들어진 상태에서 프론트엔드를 먼저 개발해야 하는 상황 — 꽤 흔한데, 이때 orval이 진짜 편함.

React Query 외에도 SWR, Vue Query, Svelte Query, Solid Query, Angular를 지원함. 프레임워크를 바꿔도 codegen 설정의 `client` 값만 바꾸면 됨.

### kubb — 플러그인 파이프라인

다른 도구들이 "이 조합을 생성해줌"이라면, kubb는 "뭘 생성할지 네가 정해"에 가까움. 타입, client, TanStack Query hook, SWR hook, Zod schema, Faker mock, MSW handler가 전부 별도 플러그인이고, 필요한 것만 골라서 조합함:

```bash
npm install -D @kubb/cli @kubb/plugin-oas @kubb/plugin-ts \
  @kubb/plugin-react-query @kubb/plugin-zod
```

```typescript
// kubb.config.ts
import { defineConfig } from "@kubb/core";
import { pluginOas } from "@kubb/plugin-oas";
import { pluginTs } from "@kubb/plugin-ts";
import { pluginReactQuery } from "@kubb/plugin-react-query";
import { pluginZod } from "@kubb/plugin-zod";

export default defineConfig({
  input: { path: "./spec.json" },
  output: { path: "./src/api/generated" },
  plugins: [
    pluginOas(),
    pluginTs({ output: { path: "types" } }),
    pluginReactQuery({ output: { path: "hooks" } }),
    pluginZod({ output: { path: "zod" } }),
  ],
});
```

```bash
npx kubb generate
```

TanStack Query 대신 SWR을 쓰고 싶으면 `@kubb/plugin-swr`로 교체. MSW mock이 필요하면 `@kubb/plugin-msw` 추가. 플러그인끼리 의존성을 선언할 수 있어서 generation 순서가 자동으로 정해짐.

npm 주간 ~72K로 다른 도구들보다 사용자가 적지만, 유연성이 필요한 프로젝트에서 선택되는 도구임.

## Redux 쓰고 있으면: RTK Query codegen

위 도구들은 전부 React Query(TanStack Query) 중심인데, Redux Toolkit을 이미 쓰고 있는 프로젝트라면 별도의 data fetching 라이브러리를 추가하는 것보다 RTK Query codegen이 자연스러움. Redux Toolkit 공식 monorepo에 포함된 도구임.

```bash
npm install -D @rtk-query/codegen-openapi
```

먼저 base API를 정의하고:

```typescript
// src/api/baseApi.ts
import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react";

export const baseApi = createApi({
  baseQuery: fetchBaseQuery({
    baseUrl: process.env.NEXT_PUBLIC_API_URL,
  }),
  endpoints: () => ({}),
});
```

codegen 설정:

```typescript
// openapi-config.ts
import type { ConfigFile } from "@rtk-query/codegen-openapi";

const config: ConfigFile = {
  schemaFile: "./spec.json",
  apiFile: "./src/api/baseApi.ts",
  outputFile: "./src/api/generated.ts",
  hooks: true,
};

export default config;
```

```bash
npx @rtk-query/codegen-openapi openapi-config.ts
```

```typescript
"use client";

import { useGetUserQuery } from "@/api/generated";

export function UserProfile({ id }: { id: string }) {
  const { data, isLoading } = useGetUserQuery({ id });
  if (isLoading) return <div>Loading...</div>;
  return <div>{data?.name}</div>;
}
```

spec에 tag 정보가 정의되어 있으면 `providesTags`/`invalidatesTags`가 자동 설정되어서 mutation 후 관련 query 캐시가 자동 무효화됨. Redux DevTools에서 API 상태를 바로 확인할 수 있다는 점도 Redux 생태계의 장점임.

## 그래서 뭘 쓰라는 건가

정답은 없지만, 판단 기준은 있음.

**Server Component 위주이고 endpoint가 많지 않으면** openapi-typescript + openapi-fetch. 가장 가볍고, 생성 코드가 타입뿐이라 black box가 없음. 번들에 추가되는 것도 거의 없음. hook boilerplate가 감당 가능한 수준이면 이게 제일 깔끔함.

**endpoint가 수십 개 이상이고 React Query hook을 전부 직접 짜기 싫으면** @hey-api/openapi-ts 또는 orval. 둘 다 hook을 자동 생성해줌. 차이는:
- 백엔드가 아직 없어서 mock이 필요하면 → orval
- 플러그인 생태계와 SDK 함수가 중요하면 → @hey-api/openapi-ts

**생성물을 세밀하게 제어하고 싶으면** kubb. 초기 설정이 좀 더 들어가지만 가장 유연함.

**Redux Toolkit 쓰고 있으면** RTK Query codegen.

## 어떤 도구를 쓰든 해야 하는 것들

### codegen script 등록

```json
{
  "scripts": {
    "codegen": "<도구별 코드젠 커맨드>",
    "postinstall": "npm run codegen"
  }
}
```

`codegen`에 들어갈 커맨드는 도구마다 다름. `npx openapi-typescript ...`일 수도 있고, `npx orval`이나 `npx @hey-api/openapi-ts`일 수도 있음. `postinstall`에 걸어두면 `npm install` 후 자동으로 코드젠이 실행됨. 새 팀원이 clone 후 install만 해도 바로 돌아가는 상태가 됨.

### CI에서 검증

```yaml
- run: npm run codegen
- run: git diff --exit-code src/api/
```

spec이 바뀌었는데 코드젠을 안 돌린 경우 CI에서 잡아줌.

생성 코드를 git에 안 넣는 전략도 있음. `.gitignore`에 생성 디렉토리를 넣고, 빌드 전에 매번 코드젠을 돌리는 방식:

```json
{
  "scripts": {
    "prebuild": "npm run codegen",
    "build": "next build"
  }
}
```

어느 쪽이든 "사람이 생성 코드를 직접 수정하지 않는다"는 원칙만 지키면 됨. 생성 결과를 바꾸고 싶으면 도구의 설정을 바꿔야지, 생성된 파일을 손으로 고치면 다음 코드젠에서 덮어써짐.

### spec 파일 관리

spec을 로컬에 복사해서 쓸 수도 있고, URL에서 직접 가져올 수도 있음. 대부분의 도구가 input에 remote URL을 지원함. @hey-api/openapi-ts 기준 예시:

```typescript
// openapi-ts.config.ts
export default defineConfig({
  input: "https://api.example.com/openapi.json",
  output: { path: "./src/api/generated" },
  plugins: ["@hey-api/typescript", "@hey-api/sdk"],
});
```

orval이라면 `input: { target: "https://api.example.com/openapi.json" }` 형태가 됨. 백엔드 배포 시 최신 spec을 가져와서 코드젠을 돌리는 CI pipeline을 구성하면 수동 복사가 필요 없어짐.

### 인증

인증 토큰 주입 같은 공통 로직은 도구마다 interceptor/middleware 패턴을 지원함. openapi-fetch 기준 예시:

```typescript
import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema";

const authMiddleware: Middleware = {
  async onRequest({ request }) {
    const token = getAccessToken();
    if (token) {
      request.headers.set("Authorization", `Bearer ${token}`);
    }
    return request;
  },
};

export const api = createClient<paths>({
  baseUrl: process.env.NEXT_PUBLIC_API_URL,
});

api.use(authMiddleware);
```

## 정리

spec이 곧 타입이고, 타입이 곧 문서임. spec에서 endpoint가 사라지거나 field 이름이 바뀌면, 코드젠 돌리는 순간 컴파일러가 영향받는 모든 곳을 에러로 알려줌. 사람이 눈으로 찾아다닐 필요가 없음. 도구는 많지만 핵심은 하나임: 사람이 타입을 손으로 관리하지 않는 것.
