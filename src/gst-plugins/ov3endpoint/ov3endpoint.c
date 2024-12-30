/*
 * (C) Copyright 2014 Kurento (http://kurento.org/)
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
#include <config.h>
#include <gst/gst.h>

#include "ov3publisher.h"
#include "ov3subscriber.h"

static gboolean
kms_ov3_endpoint_plugin_init (GstPlugin * ov3endpoint)
{
  if (!kms_ov3_publisher_plugin_init (ov3endpoint)) {
    return FALSE;
  }

  if (!kms_ov3_subscriber_plugin_init (ov3endpoint)) {
    return FALSE;
  }

  return TRUE;
}

GST_PLUGIN_DEFINE (GST_VERSION_MAJOR,
    GST_VERSION_MINOR,
    kmsov3endpoint,
    "Kurento OpenVidu3 endpoint",
    kms_ov3_endpoint_plugin_init, VERSION, GST_LICENSE_UNKNOWN,
    "Kurento OpenVidu3 endpoint", "http://www.naevatec.com")
