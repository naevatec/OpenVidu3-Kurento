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

#ifndef __OV3_PUBLISHER_IMPL_HPP__
#define __OV3_PUBLISHER_IMPL_HPP__

#include "MediaElementImpl.hpp"
#include "OV3Publisher.hpp"
#include <EventHandler.hpp>
#include <boost/property_tree/ptree.hpp>

namespace kurento
{
class OV3PublisherImpl;
} /* kurento */

namespace kurento
{
void Serialize (std::shared_ptr<kurento::OV3PublisherImpl> &object,
                JsonSerializer &serializer);
} /* kurento */

namespace kurento
{
class MediaPipelineImpl;
} /* kurento */

namespace kurento
{

class OV3PublisherImpl : public MediaElementImpl, public virtual OV3Publisher
{

  std::string url;
  std::string secret;
  std::string key;
  std::string room;
  std::string participantId;
  std::string participantName;
  bool screenShare;
  bool isConnected;
  bool publishAudio;
  bool publishVideo;

public:

  OV3PublisherImpl (const boost::property_tree::ptree &config,
                   std::shared_ptr<MediaPipeline> mediaPipeline, 
                                            const std::string &url, 
                                            const std::string &secret, 
                                            const std::string &key,
                                            const std::string &room,
                                            const std::string &participantName,
                                            const std::string &participantId,
                                            bool screenShare);

  virtual ~OV3PublisherImpl () {};

  virtual bool publishParticipant () { return publishParticipant(true, true); };
  virtual bool publishParticipant (bool publishAudio) { return publishParticipant(publishAudio, true); };
  virtual bool publishParticipant (bool publishAudio, bool publishVideo);



  virtual std::string getUrl ();
  virtual std::string getRoom ();
  virtual std::string getParticipantId ();
  virtual std::string getParticipantName ();
  virtual bool getScreenShare ();
  virtual bool getIsConnected () { return isConnected; };

  virtual void release () override;


  /* Next methods are automatically implemented by code generator */
  using MediaElementImpl::connect;
  virtual bool connect (const std::string &eventType,
                        std::shared_ptr<EventHandler> handler);
  virtual void invoke (std::shared_ptr<MediaObjectImpl> obj,
                       const std::string &methodName, const Json::Value &params,
                       Json::Value &response);

  virtual void Serialize (JsonSerializer &serializer);

protected:
  virtual void postConstructor () override;
  
private:

  class StaticConstructor
  {
  public:
    StaticConstructor();
  };

  static StaticConstructor staticConstructor;

};

} /* kurento */

#endif /*  __OV3_PUBLISHER_IMPL_HPP__ */
