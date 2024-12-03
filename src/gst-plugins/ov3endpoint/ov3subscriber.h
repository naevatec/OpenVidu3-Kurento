/*
 * (C) Copyright 2015 NaevaTec (http://www.naevatec.com/)
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
#ifndef _OV3_SUBSCRIBER_H_
#define _OV3_SUBSCRIBER_H_

#include <kurento/commons/kmselement.h>

G_BEGIN_DECLS
#define KMS_TYPE_OV3_SUBSCRIBER (ov3_subscriber_get_type())
#define KMS_OV3_SUBSCRIBER(obj) (                 \
  G_TYPE_CHECK_INSTANCE_CAST (                  \
    (obj),                                      \
    KMS_TYPE_OV3_SUBSCRIBER,                      \
    Ov3Subscriber                              \
  )                                             \
)
#define KMS_OV3_SUBSCRIBER_CLASS(klass) (         \
  G_TYPE_CHECK_CLASS_CAST (                     \
    (klass),                                    \
    KMS_TYPE_OV3_SUBSCRIBER,                      \
    KmsPassThroughClass                         \
  )                                             \
)
#define KMS_IS_OV3_SUBSCRIBER(obj) (              \
  G_TYPE_CHECK_INSTANCE_TYPE (                  \
    (obj),                                      \
    KMS_TYPE_OV3_SUBSCRIBER                       \
    )                                           \
)
#define KMS_IS_OV3_SUBSCRIBER_CLASS(klass) (      \
  G_TYPE_CHECK_CLASS_TYPE (                     \
  (klass),                                      \
  KMS_TYPE_OV3_SUBSCRIBER                         \
  )                                             \
)
typedef struct _Ov3Subscriber Ov3Subscriber;
typedef struct _Ov3SubscriberClass Ov3SubscriberClass;
typedef struct _Ov3SubscriberPrivate Ov3SubscriberPrivate;

struct _Ov3Subscriber
{
  KmsElement parent;
  Ov3SubscriberPrivate *priv;
};

struct _Ov3SubscriberClass
{
  KmsElementClass parent_class;

  /* signals */
  void (*ov3_connect) (Ov3Subscriber *obj);
  void (*ov3_disconnect) (Ov3Subscriber *obj);
  void (*ov3_request_keyframe) (Ov3Subscriber *obj);
};

GType ov3_subscriber_get_type (void);

gboolean kms_ov3_subscriber_plugin_init (GstPlugin * plugin);

G_END_DECLS
#endif  /*  _OV3_SUBSCRIBER_H_*/
