-- Seedance 视频分组复用媒体生成权限开关。
-- 新增 Seedance 分组默认关闭该能力，回填后才允许 /v1/videos/generations
-- 进入既有鉴权、额度、账号调度和供应商请求链路。
UPDATE groups
SET allow_image_generation = true
WHERE platform = 'seedance'
  AND allow_image_generation = false;
