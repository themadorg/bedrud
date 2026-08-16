# Chromium + Node for Bedrud UI screenshots.
# Build from this directory: docker build -t bedrud-screenshots .
FROM ghcr.io/puppeteer/puppeteer:24.43.1

WORKDIR /home/pptruser/app

ENV PUPPETEER_SKIP_DOWNLOAD=true
ENV BEDRUD_SCREENSHOT_OUT=/home/pptruser/app/output

COPY package.json bun.lock* package-lock.json* ./
COPY screenshot.js ./
COPY avatars/ ./avatars/

USER root
RUN npm install --omit=dev \
  && mkdir -p /home/pptruser/app/output \
  && chown -R pptruser:pptruser /home/pptruser/app
USER pptruser

# Official image caches Chrome under the pptruser home; leave executable
# discovery to Puppeteer unless the operator overrides it.

ENTRYPOINT ["node", "screenshot.js"]
CMD ["--no-start"]
