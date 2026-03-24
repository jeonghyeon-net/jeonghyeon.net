# OpenAPI spec 있는데 아직도 API 응답 타입 수동으로 만들고 있음?

백엔드한테 OpenAPI spec 파일 받았음. 근데 이걸 눈으로 읽으면서 TypeScript interface를 수동으로 만들고, endpoint마다 fetch 함수를 직접 짜고, spec 바뀔 때마다 사람이 일일이 맞추고 있음? 이 과정을 전부 자동화할 수 있음.

spec에서 타입을 뽑고, API client를 생성하고, 심지어 React Query hook이랑 MSW mock까지 자동으로 만들어주는 도구들이 있음. spec이 바뀌면 코드젠 한 번 돌리면 끝이고, 사람이 빠뜨린 건 컴파일러가 잡아줌. 사람이 수동으로 맞추면 무조건 빠뜨림. 그리고 그건 런타임에 터짐.

이게 별거 아닌 것 같은데, Stripe 출신 엔지니어 Alex Rattray가 Stainless라는 회사를 차려서 OpenAI, Anthropic, Cloudflare 공식 SDK를 이 방식으로 뽑고 있고, GitHub Octokit SDK도 OpenAPI spec에서 자동 생성함. 이미 업계 표준임.

근데 도구가 존나 많음. 하나하나 정리해봄.

## data fetching 라이브러리가 뭔데

코드젠 도구가 "hook을 생성해준다"고 하는데 그 hook이 뭔지 모르면 뒤에 나오는 거 하나도 이해 안 됨. 그래서 이거부터 함.

### 왜 필요함

`useState` + `useEffect` + `fetch`로 API 호출 직접 짜봤으면 알겠지만 이거 제대로 하려면 로딩 상태, 에러 처리, 캐시, 중복 요청 방지, refetch, 캐시 무효화, 낙관적 업데이트, 무한 스크롤 전부 직접 짜야 함. 이걸 매번 하면 코드가 개판이 됨. 그리고 이걸 버그 없이 짜는 건 생각보다 어려움.

data fetching 라이브러리가 이걸 다 해결해줌. API에서 데이터 가져오고 캐싱하고 동기화하는 걸 hook 하나로 끝냄.

### server state랑 client state

프론트엔드 상태는 두 종류임.

**server state**는 API에서 가져온 데이터임. 사용자 목록, 주문 내역 같은 거. 원본이 서버에 있고 클라이언트는 복사본을 들고 있는 거라 시간 지나면 낡고, 다른 사용자가 바꿀 수도 있고, 동기화가 필요함.

**client state**는 모달 열림닫힘, 다크 모드 토글 같은 거. 서버랑 상관없고 브라우저에서만 존재함.

예전에는 Redux에 둘 다 때려넣었는데 지금은 분리하는 게 표준임. server state는 아래 라이브러리들이, client state는 `useState`나 Zustand, Jotai 같은 게 담당함. 아직도 Redux 하나에 전부 넣고 있으면 구조를 다시 생각해봐야 함.

### TanStack Query

원래 React Query였는데 Vue, Svelte, Solid, Angular까지 지원하면서 TanStack Query로 이름 바뀜. TanStack은 만든 사람 Tanner Linsley 이름에서 따온 브랜드명임. npm 주간 ~9.5M(2025년 기준)으로 압도적 1위. 이유 없이 1위가 아님.

핵심은 `useQuery`랑 `useMutation` 두 개임.

```typescript
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";

// 데이터 가져오기
function UserProfile({ id }: { id: string }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["user", id],
    queryFn: () => fetch(`/api/users/${id}`).then((res) => res.json()),
  });

  if (isLoading) return <div>Loading...</div>;
  if (error) return <div>Error</div>;
  return <div>{data.name}</div>;
}

// 데이터 수정
function useUpdateUser(id: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (newName: string) =>
      fetch(`/api/users/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ name: newName }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["user", id] });
    },
  });
}

// 사용: mutation.mutate("새 이름")
```

`queryKey`가 캐시 키임. 같은 키를 쓰는 컴포넌트가 10개여도 API 호출은 한 번만 일어남. `invalidateQueries`로 캐시 날리면 관련 컴포넌트가 전부 알아서 refetch함. 이거 직접 구현하면 얼마나 개판이 되는지 해봤으면 알 거임.

그 외에 실무에서 자주 쓰는 기능들도 있음.

**staleTime**은 이 시간 안에는 refetch를 안 함. **gcTime**(구 cacheTime)은 캐시 유지 시간인데 지나면 메모리에서 날아감. **placeholderData**는 이전 데이터 유지하면서 새 데이터 가져올 때 쓰는 건데 페이지네이션에서 페이지 전환할 때 빈 화면 안 보이게 할 수 있음. **optimistic updates**는 `onMutate`에서 캐시를 먼저 업데이트하고 서버 요청이 실패하면 `onError`에서 롤백하는 패턴임. **infinite queries**는 `useInfiniteQuery`로 무한 스크롤 구현하는 거.

레퍼런스가 압도적으로 많음. 뭐 막히면 검색하면 거의 다 나옴. 뭘 쓸지 고민하지 말고 그냥 이거 쓰면 됨.

### SWR

Vercel이 만든 거. npm 주간 ~3.5M. 이름은 HTTP 캐시 전략 `stale-while-revalidate`에서 따온 거임.

TanStack Query보다 API가 단순함.

```typescript
"use client";

import useSWR from "swr";

const fetcher = (url: string) => fetch(url).then((res) => res.json());

function UserProfile({ id }: { id: string }) {
  const { data, error, isLoading } = useSWR(`/api/users/${id}`, fetcher);

  if (isLoading) return <div>Loading...</div>;
  if (error) return <div>Error</div>;
  return <div>{data.name}</div>;
}
```

URL이 곧 캐시 키임. queryKey 같은 거 따로 안 만들어도 됨. 이게 장점이자 단점인데 단순한 건 빠르게 되지만 복잡한 캐시 관계를 다루기엔 부족함.

mutation은 `useSWRMutation`으로 처리함.

```typescript
"use client";

import useSWRMutation from "swr/mutation";

async function updateUser(url: string, { arg }: { arg: { name: string } }) {
  return fetch(url, {
    method: "PATCH",
    body: JSON.stringify(arg),
  });
}

function useUpdateUser(id: string) {
  return useSWRMutation(`/api/users/${id}`, updateUser);
}

// 사용: trigger({ name: "새 이름" })
```

솔직히 말하면 SWR은 단순한 것만 잘 됨. API가 깔끔해서 배우기 쉽고 소규모 프로젝트에는 좋음. 근데 mutation이 약함. 낙관적 업데이트, 롤백 같은 고급 패턴은 TanStack Query가 훨씬 풍부함. 캐시 무효화도 `mutate()` 하나로 처리하는데 복잡한 캐시 관계(A 수정하면 B, C도 갱신)는 다루기 어려움. 공식 DevTools가 없어서 커뮤니티 패키지(`swr-devtools`)로 대체해야 함. Vercel이 만들었으니 Next.js랑 궁합은 좋음.

프로젝트가 복잡해질 가능성이 조금이라도 있으면 처음부터 TanStack Query 쓰는 게 나음. SWR에서 TanStack Query로 갈아타는 건 생각보다 귀찮음.

### RTK Query

Redux Toolkit에 내장되어 있음. 별도 설치 없이 `@reduxjs/toolkit`에 포함.

TanStack Query나 SWR이랑 근본적으로 다른 점이 있는데, 캐시가 Redux store 안에 들어감. 그래서 Redux DevTools에서 API 캐시 상태를 직접 볼 수 있음. 이게 다른 라이브러리에 없는 고유한 장점임.

```typescript
// src/api/baseApi.ts
import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react";

export const api = createApi({
  baseQuery: fetchBaseQuery({
    baseUrl: process.env.NEXT_PUBLIC_API_URL,
  }),
  tagTypes: ["User"],
  endpoints: (builder) => ({
    getUser: builder.query({
      query: (id: string) => `/users/${id}`,
      providesTags: (result, error, id) => [{ type: "User", id }],
    }),
    updateUser: builder.mutation({
      query: ({ id, ...body }) => ({
        url: `/users/${id}`,
        method: "PATCH",
        body,
      }),
      invalidatesTags: (result, error, { id }) => [{ type: "User", id }],
    }),
  }),
});

export const { useGetUserQuery, useUpdateUserMutation } = api;
```

```typescript
"use client";

import { useGetUserQuery } from "@/api/baseApi";

function UserProfile({ id }: { id: string }) {
  const { data, isLoading } = useGetUserQuery(id);
  if (isLoading) return <div>Loading...</div>;
  return <div>{data?.name}</div>;
}
```

`providesTags`/`invalidatesTags`로 캐시 무효화를 선언적으로 관리함. 태그 기반이라 어떤 mutation이 어떤 query를 갱신하는지 명시적으로 보임.

근데 Redux 안 쓰고 있는데 이것만을 위해 Redux를 도입하는 건 절대 하지 마라. store 설정, Provider 구성 등 보일러플레이트가 추가됨. 이미 Redux 쓰고 있으면 자연스러운 선택이고, 아니면 TanStack Query가 100배 가벼움.

### GraphQL이면 Apollo 아니면 urql

위 세 개는 전부 REST API용이고 GraphQL은 판이 다름.

**Apollo Client**가 GraphQL에서 사실상 표준임. npm 주간 ~3.1M이고 Airbnb, Shopify, The New York Times가 씀. normalized cache라는 게 특징인데, 응답을 엔티티 단위로 분해해서 저장함. `User:123`을 수정하면 이 엔티티를 참조하는 모든 query가 자동 갱신됨. 이게 잘 동작하면 마법 같은데 캐시 정규화 동작을 이해해야 해서 러닝 커브가 있음.

```typescript
"use client";

import { useQuery, gql } from "@apollo/client";

const GET_USER = gql`
  query GetUser($id: ID!) {
    user(id: $id) {
      id
      name
      email
    }
  }
`;

function UserProfile({ id }: { id: string }) {
  const { data, loading, error } = useQuery(GET_USER, {
    variables: { id },
  });

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error</div>;
  return <div>{data.user.name}</div>;
}
```

**urql**은 Apollo보다 가벼운 대안임. Formidable(현 nearForm)이 만듦. 번들이 Apollo(~50kb)의 1/3 수준(~15kb)임. Apollo의 normalized cache가 과하다 싶으면 urql의 document cache가 더 단순함. 필요하면 `@urql/exchange-graphcache`로 normalized cache를 추가할 수도 있음.

```typescript
"use client";

import { useQuery } from "urql";

const GET_USER = `
  query GetUser($id: ID!) {
    user(id: $id) {
      id
      name
      email
    }
  }
`;

function UserProfile({ id }: { id: string }) {
  const [result] = useQuery({
    query: GET_USER,
    variables: { id },
  });

  if (result.fetching) return <div>Loading...</div>;
  if (result.error) return <div>Error</div>;
  return <div>{result.data.user.name}</div>;
}
```

다만 이건 OpenAPI 코드젠이랑 상관없는 얘기임. OpenAPI는 REST spec이고 GraphQL은 자체 schema가 있음. GraphQL 코드젠은 GraphQL Code Generator라는 별도 생태계임. 참고용으로 적은 거.

### 그래서 뭘 씀

| 상황 | 답 |
|---|---|
| REST, 새 프로젝트 | TanStack Query |
| REST, 단순한 것만 | SWR |
| REST, Redux 쓰고 있음 | RTK Query |
| GraphQL, 캐시 중요 | Apollo Client |
| GraphQL, 가볍게 | urql |

고민하지 말고 TanStack Query 쓰면 됨. 단순한 GET은 `useQuery` 하나로 끝나면서 낙관적 업데이트, 무한 스크롤, 병렬 쿼리, prefetching, SSR hydration 같은 복잡한 것도 전부 됨. 레퍼런스도 압도적임.

## 본론인 코드젠 도구

여기까지가 배경이었고 여기서부터 본론임. 코드젠 도구가 뭘 하냐면 OpenAPI spec에서 위에서 설명한 라이브러리들의 hook을 자동으로 뽑아줌. 어떤 라이브러리 hook을 뽑느냐에 따라 쓸 도구가 달라짐.

- **TanStack Query** hook을 뽑고 싶으면 @hey-api/openapi-ts, orval, kubb 전부 됨
- **SWR** hook은 orval, kubb가 됨
- **RTK Query** hook은 RTK Query codegen 전용

도구별 규모는 이 정도(2025년 기준).

| 도구 | npm 주간 다운로드 | 뭘 생성하나 |
|---|---|---|
| openapi-typescript | ~2.1M | `.d.ts` 타입만. 런타임 코드 0 |
| @hey-api/openapi-ts | ~977K | SDK 함수, 타입, Zod, TanStack Query hook |
| openapi-generator | ~1.1M | 40개+ 언어. Java 기반 원조. TS 품질 별로 |
| orval | ~772K | hook, MSW mock, Zod, 타입 |
| kubb | ~72K | 전부. 플러그인 조합 |
| RTK Query codegen | ~129K | RTK Query API slice |

openapi-generator는 다운로드 수는 많은데 TS 전용으로 쓰기엔 품질이 떨어짐. Java 기반이고 생성 코드가 TypeScript 관용구에 안 맞음. Go + TS + Python을 하나의 spec에서 동시에 뽑아야 하는 polyglot 환경이 아니면 쓸 이유 없음. 나머지 다섯 개를 다룸.

## openapi-typescript + openapi-fetch

가장 많이 쓰이는 조합임. 런타임 코드를 생성하지 않는다는 게 핵심임. spec에서 `.d.ts` 타입 파일만 뽑고, openapi-fetch라는 ~6kb짜리 fetch 래퍼가 그 타입으로 type safety를 제공함. 번들에 추가되는 게 거의 없음.

```bash
npm install openapi-fetch
npm install -D openapi-typescript
```

```bash
npx openapi-typescript ./spec.json -o ./src/api/schema.d.ts
```

이러면 spec의 모든 endpoint 정보가 `paths` 타입에 담김. client에 넘기면 끝임.

```typescript
// src/api/client.ts
import createClient from "openapi-fetch";
import type { paths } from "./schema";

export const api = createClient<paths>({
  baseUrl: process.env.NEXT_PUBLIC_API_URL,
});
```

호출하면 path, query, body, response 타입이 전부 자동 추론됨.

```typescript
const { data, error } = await api.GET("/users/{id}", {
  params: {
    path: { id: "123" },
    query: { include: "profile" },
  },
});
// data, error 타입 전부 spec에서 추론됨. 잘못된 필드 넣으면 컴파일 에러.
```

Server Component에서 바로 쓸 수 있고 fetch 기반이라 Next.js 캐싱이 그대로 적용됨.

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

여기까진 좋은데 Client Component에서 TanStack Query 쓰려면 boilerplate가 생김.

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

queryKey 직접 만들고 queryFn 연결하고 select로 response 까줘야 함. endpoint 5개면 괜찮은데 50개 넘으면 진짜 고통임. 이 노가다가 싫으면 아래 도구들이 hook을 자동 생성해줌.

## @hey-api/openapi-ts

이름이 좀 이상한데 이유가 있음. 원래 `openapi-typescript-codegen`이라는 꽤 유명한 프로젝트였음. 근데 원작자(ferdikoomen)가 관리를 때려치면서 2024년 5월에 archived됨. 커뮤니티가 fork해서 `@hey-api/openapi-ts`로 이어받은 거임. 이름만 낯설지 npm 주간 977K이고 GitHub에 Vercel이랑 PayPal이 쓴다고 적혀있음. 실제로 주류 도구임.

원본이랑 다르게 플러그인 아키텍처로 재설계됨. 타입, SDK 함수, TanStack Query hook, Zod schema를 골라서 생성할 수 있음.

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

아까 openapi-fetch에서 직접 짰던 boilerplate가 사라짐.

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

endpoint마다 `getUserOptions`, `createUserMutation` 같은 option factory가 자동 생성되고 queryKey 관리도 알아서 됨. 아까 50개 endpoint에서 고통받던 게 여기서 해결됨.

Zod schema 필요하면 `@hey-api/zod` 플러그인 추가하면 됨. API response를 런타임에 검증할 수 있어서 외부 API처럼 spec이랑 실제 응답이 다를 수 있는 경우에 유용함.

## orval

TanStack Query hook 생성은 hey-api랑 비슷한데 orval의 킬러 피처는 따로 있음. MSW mock handler랑 Faker.js 기반 더미 데이터를 같이 생성해줌. 백엔드가 아직 안 만들어졌는데 프론트를 먼저 시작해야 하는 상황 자주 있지 않음? 그때 이거 쓰면 개꿀임.

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

hook 사용은 직관적임.

```typescript
"use client";

import { useGetUser } from "@/api/generated";

export function UserProfile({ id }: { id: string }) {
  const { data } = useGetUser(id);
  return <div>{data?.name}</div>;
}
```

`mock: true` 설정해뒀으니 MSW handler도 같이 나옴.

```typescript
import { getGetUserMockHandler } from "@/api/generated";
import { setupWorker } from "msw/browser";

const worker = setupWorker(getGetUserMockHandler());
worker.start();
```

spec에 정의된 타입 보고 Faker.js가 그럴듯한 더미 데이터를 넣어줌. `name` 필드에는 사람 이름이, `email` 필드에는 이메일 형식이 들어감. 백엔드 API 없이 프론트엔드 개발이 가능해짐.

TanStack Query 외에 SWR, Vue Query, Svelte Query, Solid Query, Angular도 지원함. `client` 값만 바꾸면 됨.

## kubb

다른 도구들이 "이 조합을 생성해줌"이라면 kubb는 "뭘 생성할지 네가 정해"임. 타입, hook, Zod, Faker, MSW 전부 별도 플러그인이고 필요한 것만 골라서 조합함. 가장 유연하지만 초기 설정이 좀 더 필요함.

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

SWR 쓰고 싶으면 `@kubb/plugin-swr`로 교체, MSW 필요하면 `@kubb/plugin-msw` 추가. 플러그인끼리 의존성을 선언할 수 있어서 generation 순서가 자동으로 정해짐.

npm 주간 ~72K로 위 도구들보다 사용자가 적지만 유연성이 필요한 프로젝트에서는 이게 답임.

## RTK Query codegen

Redux Toolkit 쓰고 있으면 이거. 다른 거 볼 필요 없음. Redux Toolkit 공식 monorepo에 포함된 도구임.

```bash
npm install -D @rtk-query/codegen-openapi
```

base API부터 정의함.

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

codegen 설정.

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

spec에 tag 정보가 있으면 `providesTags`/`invalidatesTags`가 자동 설정돼서 캐시 무효화도 알아서 됨.

## 뭘 쓸지 모르겠으면

**endpoint 적고 Server Component 위주면** openapi-typescript + openapi-fetch. 가장 가볍고 black box 없음.

**endpoint 많고 hook 직접 짜기 싫으면** @hey-api/openapi-ts 또는 orval. mock 필요하면 orval, 플러그인 생태계 중요하면 hey-api.

**생성물 세밀하게 제어하고 싶으면** kubb.

**Redux 쓰고 있으면** RTK Query codegen. 다른 거 볼 필요 없음.

## 세팅

어떤 도구를 골랐든 해야 하는 것들.

### codegen script

```json
{
  "scripts": {
    "codegen": "<도구별 코드젠 커맨드>",
    "postinstall": "npm run codegen"
  }
}
```

`postinstall`에 걸어두면 `npm install` 후 자동 실행됨. 새 팀원이 clone 후 install만 하면 바로 돌아감.

### CI 검증

```yaml
- run: npm run codegen
- run: git diff --exit-code src/api/
```

spec 바뀌었는데 코드젠 안 돌린 경우 CI에서 잡아줌.

생성 코드를 git에 안 넣는 전략도 있음.

```json
{
  "scripts": {
    "prebuild": "npm run codegen",
    "build": "next build"
  }
}
```

어느 쪽이든 "생성된 파일을 직접 수정하지 않는다"가 원칙임. 바꾸고 싶으면 도구 설정을 바꿔야지 생성된 파일 손대면 다음 코드젠에서 덮어써짐.

### spec 관리

로컬에 복사해서 쓸 수도 있고 URL에서 직접 가져올 수도 있음. @hey-api/openapi-ts 기준으로는 이렇게 됨.

```typescript
// openapi-ts.config.ts
export default defineConfig({
  input: "https://api.example.com/openapi.json",
  output: { path: "./src/api/generated" },
  plugins: ["@hey-api/typescript", "@hey-api/sdk"],
});
```

orval은 `input: { target: "https://..." }` 형태임. 백엔드 배포할 때 최신 spec 가져와서 코드젠 돌리는 CI pipeline 구성하면 수동 복사 안 해도 됨.

### 인증

공통 로직은 도구마다 interceptor/middleware를 지원함. openapi-fetch 기준으로는 이렇게 함.

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

## 끝

spec이 곧 타입이고 타입이 곧 문서임. spec에서 endpoint가 바뀌거나 field가 바뀌면 코드젠 돌리는 순간 컴파일러가 영향받는 곳을 전부 에러로 알려줌. spec 보면서 interface 수동으로 고치고 fetch 함수 수동으로 맞추는 짓 하지 마라.
