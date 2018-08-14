package commonutils

import (
	consulapi  "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
	"strconv"
)

func GetConsulApiClient(host string, port int) (*consulapi.Client, error) {

	log.WithFields(log.Fields{"package": "commonutils","function": "GetConsulApiClient",}).Debugf("Getting Consul API client for -> Consul.Host: %s  Consul.Port: %s", host, strconv.Itoa(port))
	config := consulapi.DefaultConfig()
	config.Address = host+":"+strconv.Itoa(port)
	return  consulapi.NewClient(config)

}

func CreateKVConsul(key string, val []byte, client *consulapi.Client) error {

	log.WithFields(log.Fields{"package": "commonutils","function": "CreateKVConsul",}).Debugf("Key: %s  Value: %s", key, string(val))
	kv := client.KV()
	// PUT a new KV pair
	p := &consulapi.KVPair{Key: key, Value: val}
	_, err := kv.Put(p, nil)
    if err != nil {
		log.WithFields(log.Fields{"package": "commonutils","function": "CreateKVConsul",}).Errorf("Error creating KV err: %s", err)
    	return  err
	}
	return nil
}

func UpdateKVTreeConsul(tree string, kvpair []*consulapi.KVPair, client *consulapi.Client) (bool, error){

	kv := client.KV()
	kvtxops := append(make([]*consulapi.KVTxnOp, 0), &consulapi.KVTxnOp{
		Verb:    consulapi.KVDeleteTree,
		Key:     tree,
	})

	for _, kv := range kvpair{
		log.WithFields(log.Fields{"package": "commonutils","function": "UpdateKVTreeConsul",}).Debugf("Key: %s  Value: %s", kv.Key, string(kv.Value))
		kvtxops = append(kvtxops, &consulapi.KVTxnOp{
			Verb:    consulapi.KVSet,
			Key:     kv.Key,
			Value:   kv.Value,
		})
	}
	ok, _, _, err := kv.Txn(kvtxops, nil)
	if err != nil{
		log.WithFields(log.Fields{"package": "commonutils","function": "UpdateKVTreeConsul",}).Errorf("Error creating bulk KV Txn err: %s", err)
	}
	return ok, err

}
