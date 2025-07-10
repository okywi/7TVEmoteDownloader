# 7TVEmoteDownloader
A little script that downloads every emote of a 7TV User using webscraping (chromedriver).

## Dependencies
You will have to install [google-chrome](https://www.google.com/chrome/) or [chromium](https://chromium.woolyss.com/download/) AND [chromedriver](https://sites.google.com/chromium.org/driver/downloads) with the **same** version.  
#### Not using a package manager:
Put the chromedriver executable in the root folder and set the path (when asked) to ```bash
./chromedriver (.\chromedriver for Windows).
```
#### Using a package manager:
Just set the path to ```bash
chromedriver
```

## Compile
### Dependencies
```
rust cargo openssl pkg-config
```

### Build
Replace yourtarget with your platform. [platforms](https://doc.rust-lang.org/nightly/rustc/platform-support.html).
```bash
cargo build --release --target yourtarget
```