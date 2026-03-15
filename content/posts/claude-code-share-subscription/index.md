# Claude Code 구독을 여러명이서 돌려쓰는 방법

![Claude Code](2026-01-07-16.27.48.webp)

계정 소유자가 아래 명령어로 토큰 생성

```bash
claude setup-token
```

발급된 토큰을 환경변수로 등록

```bash
echo "export CLAUDE_CODE_OAUTH_TOKEN=\"위에서_발급한_토큰\"" >> ~/.zshrc
```

토큰이 설정되었음에도 불구하고, Claude Code 첫 실행시 로그인을 요구하는 버그가 있어서 수기로 온보딩을 무시하도록 조치
https://github.com/anthropics/claude-code/issues/8938

```bash
vim ~/.claude.json
# /hasCompletedOnboarding    (검색)
# n                          (다음 결과로 이동, 필요시)
```

```bash
{
  "hasCompletedOnboarding": true # true로 변경 후 저장
  ...
}
```

토큰에서 구독 방식으로 돌아가려면 아래 명령어 실행

```bash
sed -i '' '/CLAUDE_CODE_OAUTH_TOKEN/d' ~/.zshrc && source ~/.zshrc
```

이후 열려있는 Claude Code 세션들을 전부 닫고, 터미널을 새로 켜던가 모든 터미널에 source ~/.zshrc 를 실행하거나.
