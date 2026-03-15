# Colima 설치

![Colima](colima.webp)

https://github.com/abiosoft/colima

```bash
brew install qemu
brew install docker
brew install colima

brew services start colima
colima start

echo 'export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock' >> ~/.zshrc
```

OrbStack가 점점 무거워져서 Colima로 이전
