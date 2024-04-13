FROM alpine:latest

# Copy the zip file from the local directory to the image
COPY ./base /pb/base
COPY ./preview_page.json /pb/preview_page.json

ENV UpdateToken="" \
    UpdateURL="" \
    website_url="" \
    email_reply_to=""
EXPOSE 8085
RUN chmod +x /pb/base

# Start PocketBase
CMD ["/pb/base", "serve", "--http=0.0.0.0:8085"]
