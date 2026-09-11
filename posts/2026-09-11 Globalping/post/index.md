# 내 서버의 국가 별 레이턴시가 얼마인지 알아보는 방법

글로벌 서비스를 하다보면 늘 드는 생각이고 반드시 해결해야 하는 문제

"유저(고객)들은 내 서비스를 이용하는데 불편함이 없을까?"
"내 서비스는 충분히 빠른가?"

태울 수 있는 돈이 많다면 상관없겠지만, 그렇지 않은 경우는 보통 서버를 한 리전에서만 띄우기 마련이다.

글로벌 서비스를 하게 되면 당연히 유저들은 해저 케이블을 태워 트래픽들을 주고 받게 될테고. 빛은 빨라도 20만 km/s 속도로 이동하며, 무수히 많은 ISP들을 거칠 것이다.

그럼 어떻게 되는가? "Hello, World!"를 즉시 응답하는 정말 단순 무식한 API든 Handshake든 뭐든 100ms 이상 걸리게 된다. 거리나 위치에 따라서 붙는 레이턴시의 편차가 크겠다만 결코 무시할 수 없을 정도로 붙는다는건 확실하다.


그래서 글로벌 게임 서버들은 [AWS Global Accelerator](https://aws.amazon.com/ko/global-accelerator/) 같은 제품들을 필히 이용하게 된다.

다만 이렇게 네트워크 경로를 최소화해도 물리적으로 거리가 멀어서 어쩔 수 없이 레이턴시가 크게 늘어나는 경우는 서버를 별도로 두게 된다. 북미 서버, 아시아 서버처럼.


웹서버는 어떤가?

대부분의 도메인들은 Query가 느리지 않으면 딱히 문제가 없다. Mutation은 느려도 심각하게 느린게 아니면 유저들이 참아주는 경우가 다반사다.

Query. 조회가 느리지 않게 하려면 어떻게 해야하는가? 간단하다. CDN을 적극적으로 활용하는 것.

미리 웹서버에서 컨텐츠들을 만든 다음 전세계 CDN으로 뿌리고 유저들은 리전별 CDN에 캐싱된 컨텐츠를 읽는 방식으로 하면 된다. (말은 쉽지, 실제로 하려고 하면 고난이다. Purge Cache..)


무슨 CDN을 쓰는게 좋을까에 대한 고찰은 어떻게 하면 좋을까?

Cloudflare, Amazon Cloudfront, Akamai, Fastly 등등 CDN은 무척이나 많다.


나는 가성비도 중요하지만 성능이 최우선이어야 한다고 생각하기 때문에 레이턴시와 네트워크 경로가 얼마나 축약되어있는지를 알아보는 테스트를 준비해보기로 했고.

전 세계에 람다를 배포하거나, 공짜 VPN을 이용해서 트래픽을 날려보는 테스트를 하면 되겠다 싶어서 AI Agent를 열심히 돌리는데 뜬금없이 좋은 공짜 서비스가 있다길래 알아봤다.

- Globalping
    - [Website](https://globalping.io/)
    - [GitHub](https://github.com/jsdelivr/globalping)

무려 Ping, Traceroute, MTR, DNS, HTTP를 120개 국가에서 날려볼 수 있는 공짜! 서비스더라.

curl로 호출할 수 있는 api도 있고, cli도 있다.

```bash
curl -X POST 'https://api.globalping.io/v1/measurements' \
  -H 'Content-Type: application/json' \
  -H 'User-Agent: my-globalping-test/1.0' \
  -d '{
    "type": "ping",
    "target": "example.com",
    "locations": [
      {
        "magic": "Japan"
      }
    ],
    "limit": 5
  }'
```

```bash
brew tap jsdelivr/globalping
brew install globalping

# 전 세계에서 최대 20개 프로브로 HTTP GET 측정
globalping http https://example.com from world --limit 20 --method get

# 일본의 일반 인터넷 접속망에서 측정
globalping http https://example.com from Japan+eyeball-network \
  --limit 5 --method get

# 시간 정보 중심으로 출력
globalping http https://example.com from world \
  --limit 10 --method get --latency
```

이걸 AI Agent 한테 잘 던져가지고 보고서 만들어달라고 하면 될 것 같다.

아주 좋군?
