cd /home/ranxuejiao/huola-blog
rm -rf .deploy_git
hexo generate
cd .deploy_git
git init
git remote add origin git@github.com:huola0328/huola0328.github.io.git
git add .
git commit -m "deploy butterfly theme"
git push -u origin master --force
cd ..
