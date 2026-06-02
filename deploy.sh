#!/bin/bash
set -euo pipefail

cd /home/ranxuejiao/huola-blog
npx hexo clean
npx hexo generate
cd public
git init
git checkout -b clean_deploy
git remote add origin git@github.com:huola0328/huola0328.github.io.git
git add .
git commit -m "deploy $(date '+%Y-%m-%d %H:%M:%S')"
git push -u origin clean_deploy --force
cd ..
echo "部署完成！访问 https://huola0328.github.io"
