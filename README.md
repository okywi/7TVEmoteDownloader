# 7TVEmoteDownloader
A little script that downloads every emote of a 7TV User using webscraping (chromedriver).

## Dependencies
You will have to install [google-chrome](https://www.google.com/chrome/) or [chromium](https://chromium.woolyss.com/download/) AND [chromedriver](https://sites.google.com/chromium.org/driver/downloads) with the **same** version.

## Compile
### Dependencies
```
rust
cargo
openssl
pkg-config
```

### Build
Replace your target with one of these [platforms](https://doc.rust-lang.org/nightly/rustc/platform-support.html).  
```cargo build --release --target yourtarget```