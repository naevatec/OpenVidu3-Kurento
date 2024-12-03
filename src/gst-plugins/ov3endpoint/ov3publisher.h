/*
 * (C) Copyright 2015 Kurento (http://kurento.org/)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
#ifndef _OV3_PUBLISHER_H_
#define _OV3_PUBLISHER_H_

#include <kurento/commons/kmselement.h>

G_BEGIN_DECLS
#define KMS_TYPE_OV3_PUBLISHER (ov3_publisher_get_type())
#define KMS_OV3_PUBLISHER(obj) (                 \
  G_TYPE_CHECK_INSTANCE_CAST (                  \
    (obj),                                      \
    KMS_TYPE_OV3_PUBLISHER,                      \
    Ov3Publisher                              \
  )                                             \
)
#define KMS_OV3_PUBLISHER_CLASS(klass) (         \
  G_TYPE_CHECK_CLASS_CAST (                     \
    (klass),                                    \
    KMS_TYPE_OV3_PUBLISHER,                      \
    KmsPassThroughClass                         \
  )                                             \
)
#define KMS_IS_OV3_PUBLISHER(obj) (              \
  G_TYPE_CHECK_INSTANCE_TYPE (                  \
    (obj),                                      \
    KMS_TYPE_OV3_PUBLISHER                       \
    )                                           \
)
#define KMS_IS_OV3_PUBLISHER_CLASS(klass) (      \
  G_TYPE_CHECK_CLASS_TYPE (                     \
  (klass),                                      \
  KMS_TYPE_OV3_PUBLISHER                         \
  )                                             \
)

typedef struct _Ov3Publisher Ov3Publisher;
typedef struct _Ov3PublisherClass Ov3PublisherClass;
typedef struct _Ov3PublisherPrivate Ov3PublisherPrivate;

struct _Ov3Publisher
{
  KmsElement parent;
  Ov3PublisherPrivate *priv;
};

struct _Ov3PublisherClass
{
  KmsElementClass parent_class;

  /* signals */
  void (*ov3_connect) (Ov3Publisher *obj);
  void (*ov3_disconnect) (Ov3Publisher *obj);
};

GType ov3_publisher_get_type (void);

gboolean kms_ov3_publisher_plugin_init (GstPlugin * plugin);

G_END_DECLS
#endif /*_OV3_PUBLISHER_H_*/
