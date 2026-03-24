# OpenAPI Spec 기반 Next.js 프론트엔드 개발: 코드젠 도구부터 data fetching 라이브러리까지

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

코드젠 도구들이 "hook을 생성한다"고 할 때, 그 hook은 결국 어떤 data fetching 라이브러리의 hook임. 이 라이브러리들이 뭔지 모르면 뒤의 내용이 붕 뜨니까, 여기서 짚고 가겠음.

### 왜 필요한가

`useState` + `useEffect` + `fetch`로 API 호출을 직접 짜면 이런 걸 전부 손으로 처리해야 함:

- 로딩/에러 상태 관리
- 캐시 (같은 데이터를 여러 컴포넌트에서 요청할 때 중복 호출 방지)
- 백그라운드 refetch (탭 전환 시 최신 데이터로 갱신)
- 캐시 무효화 (mutation 후 관련 데이터 다시 가져오기)
- 낙관적 업데이트 (서버 응답 전에 UI를 먼저 갱신)
- 페이지네이션, 무한 스크롤

이걸 매번 직접 구현하면 코드가 비대해지고 버그가 생김. data fetching 라이브러리는 이런 공통 문제를 추상화해서 해결해줌.

### server state vs client state

한 가지 중요한 구분이 있음. 프론트엔드에서 다루는 상태는 크게 두 종류임.

**server state**: API에서 가져온 데이터. 사용자 목록, 게시물 내용, 주문 내역 등. 원본은 서버에 있고, 클라이언트는 복사본을 가지고 있는 거임. 시간이 지나면 낡을 수 있고(stale), 다른 사용자가 변경할 수 있고, 동기화가 필요함.

**client state**: 브라우저에서만 존재하는 상태. 모달 열림/닫힘, 다크 모드 토글, 폼 입력값, 사이드바 펼침/접힘 등. 서버와 무관하고, 동기화할 대상이 없음.

이 둘은 성격이 완전히 다른데, 예전에는 Redux 같은 하나의 전역 상태 관리 도구에 전부 넣었음. 지금은 분리하는 게 표준임. server state는 아래에서 설명할 라이브러리들이 담당하고, client state는 `useState`, Zustand, Jotai 같은 도구가 담당함.

### TanStack Query (React Query)

가장 널리 쓰이는 server state 관리 라이브러리. 원래 React Query라는 이름이었는데, Tanner Linsley(개발자 이름)가 React 외에 Vue, Svelte, Solid, Angular까지 지원하면서 TanStack Query로 이름을 바꿨음. TanStack은 이 개발자의 오픈소스 프로젝트 브랜드명임(TanStack Table, TanStack Router 등도 있음).

핵심 API는 `useQuery`와 `useMutation` 두 개임:

```typescript
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";

// 데이터 조회
function UserProfile({ id }: { id: string }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["user", id],
    queryFn: () => fetch(`/api/users/${id}`).then((res) => res.json()),
  });

  if (isLoading) return <div>Loading...</div>;
  if (error) return <div>Error</div>;
  return <div>{data.name}</div>;
}

// 데이터 변경
function UpdateButton({ id }: { id: string }) {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: (newName: string) =>
      fetch(`/api/users/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ name: newName }),
      }),
    onSuccess: () => {
      // mutation 성공 후 관련 캐시를 무효화해서 최신 데이터를 다시 가져옴
      queryClient.invalidateQueries({ queryKey: ["user", id] });
    },
  });

  return <button onClick={() => mutation.mutate("새 이름")}>수정</button>;
}
```

`queryKey`는 캐시의 키임. 같은 `["user", "123"]`을 쓰는 컴포넌트가 여러 개 있어도 API 호출은 한 번만 일어나고, 결과를 공유함. `invalidateQueries`로 특정 키의 캐시를 무효화하면 해당 데이터를 쓰는 모든 컴포넌트가 자동으로 refetch함.

그 외 기능:
- **staleTime**: 데이터가 "신선한" 상태로 유지되는 시간. 이 시간 안에는 refetch를 안 함.
- **gcTime** (구 cacheTime): 캐시가 메모리에 남아있는 시간. 이 시간이 지나면 가비지 컬렉션됨.
- **placeholderData**: 이전 데이터를 유지하면서 새 데이터를 가져올 때 사용. 페이지네이션에서 페이지 전환 시 빈 화면이 안 보이게 할 수 있음.
- **optimistic updates**: `onMutate`에서 캐시를 먼저 업데이트하고, 서버 요청이 실패하면 `onError`에서 롤백하는 패턴.
- **infinite queries**: `useInfiniteQuery`로 무한 스크롤 구현.

### SWR

Vercel이 만든 data fetching 라이브러리. 이름은 HTTP 캐시 전략 `stale-while-revalidate`에서 옴. 캐시된(stale) 데이터를 먼저 보여주고, 백그라운드에서 최신 데이터를 가져와서(revalidate) 교체하는 방식임.

TanStack Query와 같은 문제를 풀지만 API가 더 단순함:

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

`useSWR`에 URL과 fetcher 함수를 넘기면 끝임. URL이 곧 캐시 키 역할을 함. TanStack Query처럼 `queryKey`와 `queryFn`을 분리해서 설정할 필요가 없음.

mutation은 `useSWRMutation`으로 처리함:

```typescript
"use client";

import useSWRMutation from "swr/mutation";

async function updateUser(url: string, { arg }: { arg: { name: string } }) {
  return fetch(url, {
    method: "PATCH",
    body: JSON.stringify(arg),
  });
}

function UpdateButton({ id }: { id: string }) {
  const { trigger } = useSWRMutation(`/api/users/${id}`, updateUser);
  return <button onClick={() => trigger({ name: "새 이름" })}>수정</button>;
}
```

TanStack Query와의 차이:
- **API 단순함**: 설정이 적고 배우기 쉬움. 소규모 프로젝트에 적합함.
- **mutation이 약함**: `useSWRMutation`이 있긴 하지만, TanStack Query의 `useMutation`만큼 낙관적 업데이트, 롤백 같은 고급 패턴 지원이 풍부하지 않음.
- **캐시 무효화가 단순함**: `mutate()` 하나로 처리. 유연하지만, 복잡한 캐시 관계(A를 수정하면 B, C도 갱신)를 관리하기엔 TanStack Query의 `invalidateQueries`가 더 편함.
- **공식 DevTools 없음**: TanStack Query는 공식 DevTools로 캐시 상태를 시각적으로 확인할 수 있음. SWR은 공식 DevTools가 없고, 커뮤니티 패키지(`swr-devtools`)로 대체해야 함.
- **Next.js 친화적**: Vercel이 만들었으니 Next.js와의 궁합이 좋음. 특히 App Router의 서버 사이드 data fetching과 자연스럽게 조합됨.

### RTK Query

Redux Toolkit에 내장된 data fetching 솔루션. 별도 패키지를 설치할 필요 없이 `@reduxjs/toolkit`에 포함되어 있음.

위의 두 라이브러리와 근본적으로 다른 점이 있음. TanStack Query와 SWR은 독립적인 캐시를 가지고 있는데, RTK Query는 Redux store 안에 캐시를 저장함. 그래서 Redux DevTools에서 API 캐시 상태를 직접 확인할 수 있음.

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

`providesTags`/`invalidatesTags`로 캐시 무효화를 선언적으로 관리함. `updateUser` mutation이 성공하면 같은 `User` 태그를 가진 query가 자동으로 refetch됨. 이 패턴이 명시적이라 대규모 프로젝트에서 캐시 관계를 추적하기 편함.

다만 Redux Toolkit을 안 쓰고 있는 프로젝트에서 RTK Query만을 위해 Redux를 도입하는 건 과함. Redux store 설정, Provider 구성 등 보일러플레이트가 추가됨. 이미 Redux를 쓰고 있다면 자연스러운 선택이고, 아니라면 TanStack Query나 SWR이 더 가벼움.

### GraphQL이라면: Apollo Client, urql

위 세 라이브러리는 전부 REST API 기반인데, GraphQL을 쓰고 있다면 선택지가 다름.

**Apollo Client**: GraphQL 생태계에서 가장 널리 쓰이는 client. TanStack Query가 REST에서 차지하는 위치를 GraphQL에서 차지하고 있음. normalized cache(응답을 엔티티 단위로 분해해서 저장)가 특징인데, `User:123`을 수정하면 이 엔티티를 참조하는 모든 query가 자동으로 갱신됨. 캐시 무효화를 명시적으로 할 필요가 줄어드는 대신, 캐시 정규화 동작을 이해해야 함.

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

**urql**: Apollo보다 가벼운 GraphQL client. Formidable(현 nearForm)이 만듦. Apollo의 normalized cache가 과하다고 느끼면 urql의 document cache(query 단위 캐시)가 더 단순함. 번들 사이즈도 Apollo(~50kb)의 약 1/3 수준(~15kb)임. 필요하면 `@urql/exchange-graphcache`로 normalized cache를 추가할 수 있음.

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

GraphQL 프로젝트에서의 선택 기준: 복잡한 캐시 정규화가 필요하면 Apollo, 단순하고 가볍게 가고 싶으면 urql.

다만 이 글의 주제인 OpenAPI 코드젠과는 직접 관련이 없음. OpenAPI는 REST API spec이고, GraphQL은 자체 schema 언어가 있음. GraphQL 코드젠은 GraphQL Code Generator라는 별도의 생태계가 있음.

### 누가 뭘 쓰나

공개적으로 확인 가능한 사용 사례:

**TanStack Query**: npm 주간 다운로드 ~9.5M(2025년 기준)으로 압도적 1위. 공식 문서가 방대하고, 블로그 포스트, 강의, 커뮤니티 리소스도 가장 많음. Vercel의 Next.js 공식 예제에서도 TanStack Query를 data fetching 라이브러리로 사용하는 예시가 포함되어 있음.

**SWR**: Vercel이 만들었고, Next.js 공식 문서에서 client-side data fetching 예시로 직접 소개함. npm 주간 ~3.5M. Vercel 자체 프로덕트에서도 사용됨.

**RTK Query**: Redux Toolkit의 일부이고, Redux는 여전히 대규모 엔터프라이즈 프로젝트에서 많이 쓰임. Meta의 여러 내부 도구, Spotify의 웹 앱 등 Redux 기반 프로젝트에서 RTK Query를 채택하는 추세임.

**Apollo Client**: Airbnb, Shopify, The New York Times 등 GraphQL을 도입한 기업에서 사실상 표준으로 쓰임. npm 주간 ~3.1M.

**urql**: Apollo보다 작은 규모지만 Formidable(현 nearForm) 외에도 여러 스타트업에서 사용 중.

### 결론부터 말하면: TanStack Query

"가장 단순하면서도 복잡한 것까지 다 되는 게 뭐냐"라는 질문에는 TanStack Query가 답임.

단순한 GET 요청은 `useQuery` 하나로 끝나고, 낙관적 업데이트, 무한 스크롤, 병렬 쿼리, dependent 쿼리, prefetching, SSR hydration 같은 복잡한 시나리오도 전부 커버함. 공식 문서가 가장 방대하고, Stack Overflow, 블로그, YouTube 강의 등 레퍼런스가 압도적으로 많음. 뭔가 막히면 검색하면 거의 다 나옴.

SWR은 더 단순하지만 "단순한 것만 잘 됨". mutation 쪽이 약하고 복잡한 캐시 관계를 다루기 어려움. RTK Query는 강력하지만 Redux 생태계에 묶여 있음. 특별한 이유가 없으면 TanStack Query로 시작하는 게 가장 안전한 선택임.

### 정리: 어떤 라이브러리를 고를 것인가

| 상황 | 선택 |
|---|---|
| REST API, 새 프로젝트 | TanStack Query |
| REST API, 단순한 data fetching 위주 | SWR |
| REST API, Redux 이미 쓰고 있음 | RTK Query |
| GraphQL, 복잡한 캐시 필요 | Apollo Client |
| GraphQL, 가볍게 가고 싶음 | urql |

REST API + Next.js 조합이면 사실상 **TanStack Query vs SWR** 양자택일이고, Redux 쓰고 있으면 RTK Query가 추가 선택지임. 확신이 없으면 TanStack Query.

### 코드젠과의 관계

이 라이브러리들의 hook을 직접 짜는 대신, 코드젠 도구가 OpenAPI spec에서 자동으로 생성해주는 거임. 어떤 라이브러리의 hook을 생성하느냐에 따라 코드젠 도구 선택이 달라짐:

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
