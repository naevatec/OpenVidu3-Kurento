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

#ifndef __OV3_SUBSCRIBER_IMPL_HPP__
#define __OV3_SUBSCRIBER_IMPL_HPP__

#include "MediaElementImpl.hpp"
#include "OV3Subscriber.hpp"
#include <EventHandler.hpp>
#include <boost/property_tree/ptree.hpp>

namespace kurento
{
class OV3SubscriberImpl;
} /* kurento */

namespace kurento
{
void Serialize (std::shared_ptr<kurento::OV3SubscriberImpl> &object,
                JsonSerializer &serializer);
} /* kurento */

namespace kurento
{
class MediaPipelineImpl;
} /* kurento */

namespace kurento
{

class OV3SubscriberImpl : public MediaElementImpl, public virtual OV3Subscriber
{

  std::string url;
  std::string secret;
  std::string key;
  std::string room;
  std::string participantId;
  bool screenShare;
  bool isConnected;

public:

  OV3SubscriberImpl (const boost::property_tree::ptree &config,
                   std::shared_ptr<MediaPipeline> mediaPipeline, 
                                            const std::string &url, 
                                            const std::string &secret, 
                                            const std::string &key);

  virtual ~OV3SubscriberImpl () {};

  virtual bool subscribeParticipant (const std::string &room, const std::string &participantId, bool screenShare);
  virtual void requestKeyFrame ();

  virtual std::string getUrl ();
  virtual std::string getRoom ();
  virtual std::string getParticipantId ();
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

#endif /*  __OV3_SUBSCRIBER_IMPL_HPP__ */
