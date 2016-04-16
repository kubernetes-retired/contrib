# git-sync

git-sync is a command that pull a git repository to a local directory.

It can be used to source a container volume with the content of a git repo.

## Usage

```
# build the container
docker build -t git-sync .
# run the git-sync container
docker run -d GIT_SYNC_REPO=https://github.com/GoogleCloudPlatform/kubernetes GIT_SYNC_DEST=/git -e GIT_SYNC_BRANCH=gh-pages -r HEAD GIT_SYNC_DEST=/git -v /git-data:/git git-sync
# run a nginx container to serve sync'ed content
docker run -d -p 8080:80 -v /git-data:/usr/share/nginx/html nginx 
```

[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/contrib/git-sync/README.md?pixel)]()
