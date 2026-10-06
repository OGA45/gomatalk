FROM golang:1.25.3-trixie as builder

RUN mkdir -p /workspace
WORKDIR /workspace

COPY .  /workspace/.

# C コンパイラ + pkg-config（cgo: gopus / go-sqlite3 / libdave 用）。
# curl/unzip/cmake/ninja/git は Discord 公式 libdave（DAVE E2EE）導入用。
# mp3/oto/mpg123 依存はコード側から除去済みのため libmpg123/libasound は不要。
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential pkg-config ca-certificates curl unzip git cmake ninja-build && \
    rm -rf /var/lib/apt/lists/*

# Discord 公式 libdave v1.1.0（ビルド済み BoringSSL 版）を godave のインストーラで
# /root/.local に導入する（dave.h / libdave.so / pkg-config メタデータを提供）。
RUN curl -sSL https://raw.githubusercontent.com/disgoorg/godave/master/scripts/libdave_install.sh -o /tmp/libdave_install.sh && \
    bash /tmp/libdave_install.sh v1.1.0 < /dev/null

ENV PKG_CONFIG_PATH=/root/.local/lib/pkgconfig
ENV LD_LIBRARY_PATH=/root/.local/lib
ENV CGO_ENABLED=1

RUN go mod tidy
RUN go build

FROM debian:trixie

# --no-install-recommends でなければ ffmpeg が mesa/gtk/x11 等 GUI 系の推奨
# パッケージを大量に巻き込み 350+ パッケージ・1GB 超に膨張する。ヘッドレス用途では不要。
# build-essential は実行に不要（バイナリは builder で生成済み）、libsqlite3-dev も
# go-sqlite3 が sqlite を静的に取り込むため不要。
# libstdc++6 は libdave.so（C++）の実行時依存。
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates unzip ffmpeg wget open-jtalk open-jtalk-mecab-naist-jdic libstdc++6 && \
    apt-get upgrade -y && \
    rm -rf /var/lib/apt/lists/*
RUN mkdir -p /workspace
WORKDIR /workspace

RUN \
 mkdir -p /usr/share/open_jtalk/voices && \
 wget http://downloads.sourceforge.net/open-jtalk/hts_voice_nitech_jp_atr503_m001-1.05.tar.gz && \
 tar -zxvf hts_voice_nitech_jp_atr503_m001-1.05.tar.gz && \
 cp hts_voice_nitech_jp_atr503_m001-1.05/*  /usr/share/open_jtalk/voices/. && \
 rm -rf hts_voice_nitech_jp_atr503_m001-1.05*

RUN \
 mkdir -p /usr/share/open_jtalk/voices && \
 wget https://downloads.sourceforge.net/project/mmdagent/MMDAgent_Example/MMDAgent_Example-1.8/MMDAgent_Example-1.8.zip && \
 unzip MMDAgent_Example-1.8.zip && \
 cp MMDAgent_Example-1.8/Voice/mei/* /usr/share/open_jtalk/voices/. && \
 rm -rf MMDAgent_Example-1.8*


RUN apt-get purge -y --auto-remove wget unzip
RUN apt-get clean autoclean
RUN apt-get autoremove --yes
RUN rm -rf /var/lib/{apt,dpkg,cache,log}/


# libdave.so（DAVE E2EE）。Go バイナリはこれに動的リンクされている。
COPY --from=builder /root/.local/lib/libdave.so /usr/local/lib/libdave.so
RUN ldconfig
ENV LD_LIBRARY_PATH=/usr/local/lib

COPY --from=builder /workspace/gomatalk .
RUN mkdir data
VOLUME /workspace/data
RUN mkdir wav
VOLUME /workspace/wav
RUN mkdir voices
VOLUME /workspace/voices
RUN mkdir migrates
COPY migrates  /workspace/migrates/.

# Discord Activity backend HTTP server (pkg/activity). Default listen :8080.
EXPOSE 8080

CMD ["/workspace/gomatalk", "-f", "/workspace/config/config.toml"]
