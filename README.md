# gotools

##gifdecode 
将gif和mp4文件进行抽帧，并合成大图片，方便对gif或mp4内容的快速预览
###启动
支持的启动选项：
$./gifdecode -h
Usage of ./gifdecode:
  -cache string
    	The cache dir of gif and mp4,make sure the directory can be writed. (default "./cache")
  -expire int
    	The days expire of cache files (default 3)
  -ffmpeg string
    	The bin of ffmpeg,make sure it is in the PATH (default "ffmpeg")
  -port int
    	The Listen Port (default 9100)

在后台运行
$./gifdecode >> run.log 2>&1 &

### 请求方式
####gif示例
 http://localhost:9100/gif?src=http://p4.so.qhimg.com/t012c912660f1990c49.gif
参数列表:
src:gif图片地址，做urlencode下
refresh: 1强制刷新，其他值表示走cache模式
quality:图片品质，缺省是80
width: 合并后一行图片里多张图片总和的最大宽度

####Mp4示例
http://localhost:9100/mp4?src=http%3a%2f%2fus.sinaimg.cn%2f004nAoHQjx070CWyKRUs050401009Sly0k01.mp4%3fKID%3dunistore%2cvideo%26Expires%3d1460365492%26ssig%3dX6FmeNexYK
src:mp4视频地址，做urlencode下
refresh: 1强制刷新，其他值表示走cache模式
width: 合并后一行图片里多张图片总和的最大宽度


