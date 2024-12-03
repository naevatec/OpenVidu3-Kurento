/*
 * (C) Copyright 2016 Kurento (http://kurento.org/)
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

#include <gst/gst.h>
#include "MediaPipelineImpl.hpp"
#include <jsonrpc/JsonSerializer.hpp>
#include <KurentoException.hpp>

#include "OV3SubscriberImpl.hpp"
#include <OV3SubscriberImplFactory.hpp>

#define GST_CAT_DEFAULT ov3_subscriber_impl
GST_DEBUG_CATEGORY_STATIC (GST_CAT_DEFAULT);
#define GST_DEFAULT_NAME "OV3SubscriberImpl"

#define FACTORY_NAME "kmsov3subscriber"

namespace kurento
{

OV3SubscriberImpl::OV3SubscriberImpl (const boost::property_tree::ptree &config,
                                  std::shared_ptr<MediaPipeline> mediaPipeline, 
                                            const std::string &_url, 
                                            const std::string &_secret, 
                                            const std::string &_key)  : MediaElementImpl (config,
                                        std::dynamic_pointer_cast<MediaObjectImpl> (mediaPipeline), FACTORY_NAME),
                                        url (_url), secret (_secret), key (_key), isConnected (false)

{
}

MediaObjectImpl *
OV3SubscriberImplFactory::createObject (const boost::property_tree::ptree &conf, 
                                            std::shared_ptr<MediaPipeline> mediaPipeline, 
                                            const std::string &url, 
                                            const std::string &secret, 
                                            const std::string &key) const
{
  return new OV3SubscriberImpl (conf, mediaPipeline, url, secret, key);
}

std::string 
OV3SubscriberImpl::getUrl ()
{
  return url;
}

std::string 
OV3SubscriberImpl::getRoom ()
{
  return room;
}

std::string 
OV3SubscriberImpl::getParticipantId ()
{
  return participantId;
}

bool 
OV3SubscriberImpl::getScreenShare ()
{
  return screenShare;
}


void OV3SubscriberImpl::postConstructor ()
{
  MediaElementImpl::postConstructor ();

}

void OV3SubscriberImpl::release ()
{
  g_signal_emit_by_name (element, "ov3-disconnect");

  MediaElementImpl::release ();
}

bool OV3SubscriberImpl::subscribeParticipant (const std::string &room, const std::string &participantId, bool screenShare)
{
  if (this->isConnected) {
        throw KurentoException (SDP_END_POINT_ALREADY_NEGOTIATED,
                            "Endpoint already negotiated");
  }
  this->room = room;
  this->participantId = participantId;
  this->screenShare = screenShare;

  g_object_set (element, "ov3-url", url.c_str(), 
                         "ov3-secret", secret.c_str(),
                         "ov3-key", key.c_str(), 
                         "ov3-room", room.c_str(), 
                         "ov3-participant", participantId.c_str(), 
                         "ov3-screenshare", screenShare, NULL); 

  g_signal_emit_by_name (element, "ov3-connect");

  g_object_get (element, "ov3-connected", &isConnected, NULL);

  return isConnected;
}

void 
OV3SubscriberImpl::requestKeyFrame ()
{
  if (isConnected) {
    g_signal_emit_by_name (element, "ov3-request-keyframe");
  }
}


OV3SubscriberImpl::StaticConstructor OV3SubscriberImpl::staticConstructor;

OV3SubscriberImpl::StaticConstructor::StaticConstructor()
{
  GST_DEBUG_CATEGORY_INIT (GST_CAT_DEFAULT, GST_DEFAULT_NAME, 0,
                           GST_DEFAULT_NAME);
}

} /* kurento */
