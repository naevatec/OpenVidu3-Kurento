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

#include "OV3PublisherImpl.hpp"
#include <OV3PublisherImplFactory.hpp>

#define GST_CAT_DEFAULT ov3_subscriber_impl
GST_DEBUG_CATEGORY_STATIC (GST_CAT_DEFAULT);
#define GST_DEFAULT_NAME "OV3PublisherImpl"

#define FACTORY_NAME "kmsov3publisher"

namespace kurento
{

OV3PublisherImpl::OV3PublisherImpl (const boost::property_tree::ptree &config,
                                  std::shared_ptr<MediaPipeline> mediaPipeline, 
                                            const std::string &_url, 
                                            const std::string &_secret, 
                                            const std::string &_key,
                                            const std::string &_room,
                                            const std::string &_participantName,
                                            const std::string &_participantId,
                                            bool _screenShare)  : MediaElementImpl (config,
                                        std::dynamic_pointer_cast<MediaObjectImpl> (mediaPipeline), FACTORY_NAME),
                                        url (_url), secret (_secret), key (_key), room(_room), participantId(_participantId), 
                                        participantName(_participantName), screenShare(_screenShare), isConnected(false)

{
}

MediaObjectImpl *
OV3PublisherImplFactory::createObject (const boost::property_tree::ptree &conf, 
                                            std::shared_ptr<MediaPipeline> mediaPipeline, 
                                            const std::string &url, 
                                            const std::string &secret, 
                                            const std::string &key, 
                                            const std::string &room, 
                                            const std::string &participantName, 
                                            const std::string &participantId, 
                                            bool screenShare) const
{
  return new OV3PublisherImpl (conf, mediaPipeline, url, secret, key, room, participantName, participantId, screenShare);
}

std::string 
OV3PublisherImpl::getUrl ()
{
  return url;
}

std::string 
OV3PublisherImpl::getRoom ()
{
  return room;
}

std::string 
OV3PublisherImpl::getParticipantId ()
{
  return participantId;
}

std::string 
OV3PublisherImpl::getParticipantName ()
{
  return participantName;
}

bool 
OV3PublisherImpl::getScreenShare ()
{
  return screenShare;
}


void OV3PublisherImpl::postConstructor ()
{
  MediaElementImpl::postConstructor ();

}

void OV3PublisherImpl::release ()
{
  g_signal_emit_by_name (element, "ov3-disconnect");

  MediaElementImpl::release ();
}

bool OV3PublisherImpl::publishParticipant (bool pubAudio, bool pubVideo)
{
  if (this->isConnected) {
        throw KurentoException (SDP_END_POINT_ALREADY_NEGOTIATED,
                            "Endpoint already negotiated");
  }

  g_object_set (element, "ov3-url", url.c_str(), 
                         "ov3-secret", secret.c_str(),
                         "ov3-key", key.c_str(), 
                         "ov3-room", room.c_str(), 
                         "ov3-participant-name", participantName.c_str(), 
                         "ov3-participant-id", participantId.c_str(), 
                         "ov3-screenshare", this->screenShare, 
                         "ov3-publishAudio", pubAudio,
                         "ov3-publishVideo", pubVideo, NULL); 

  g_signal_emit_by_name (element, "ov3-connect");

  g_object_get (element, "ov3-connected", &isConnected, NULL);

  return isConnected;
}


OV3PublisherImpl::StaticConstructor OV3PublisherImpl::staticConstructor;

OV3PublisherImpl::StaticConstructor::StaticConstructor()
{
  GST_DEBUG_CATEGORY_INIT (GST_CAT_DEFAULT, GST_DEFAULT_NAME, 0,
                           GST_DEFAULT_NAME);
}

} /* kurento */
