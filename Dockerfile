FROM alpine:latest

# Copy the zip file from the local directory to the image
COPY ./base /pb/base
COPY ./preview_page.json /pb/preview_page.json

ENV UpdateToken="" \
    UpdateURL="" \
    website_url="" \
    email_reply_to=""\
    port="8085"
RUN chmod +x /pb/base
#Expose the default port
EXPOSE 8085
# Start PocketBase
CMD ["/bin/sh", "-c", "/pb/base serve --http=0.0.0.0:$port"]
