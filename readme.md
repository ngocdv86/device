Install
```sh
#https://goreleaser.com/install
brew install goreleaser
goreleaser init
```

Build local
```sh
#https://goreleaser.com/quick-start/
goreleaser release --snapshot --clean 
```

Build and publish to remote repository
```
git tag -a v1.x.y m "message"
git push origin v1.x.y
goreleaser release --clean 
```

Run
- MacOS
    - Right click to `device` app, click Open. 
- Linux
    - Open terminal, enter `sudo ./device`
- Windows
    - Right click to `device` app, Run as Administrator