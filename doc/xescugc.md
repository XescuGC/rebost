# Gogilefs

### Index

* [Objective](#objective)
* [Implementation](#implementation)
* [Configuration](#configuration)
* [Objects Stored](#objects_stored)
* [Term](#term)
* [Node (follower) role](#node_role)
  - [Store](#store)
  - [Serve](#serve)
  - [Status](#status)
  - [Comunication](#comunication)
  - [Replication](#node_replication)
* [Leader role](#leader_role)
  - [Replication](#leader_replication)
  - [Leader election](#leader_election)
  - [New Node](#new_node)

## Objective

The objective is to write a Distributed filesystem inspired in MogileFS

## Implementation

The main idea behind it, is to be EASY to setup (barely no configuration needed). We plan to implement a Leader/Follower distribution for the Nodes, but in this case the Leaders is a week Leader. Each Node has a KV store of the Objects it knows (not the full DB) and where they are replicated.

Each Object(stored) has a Class/Type/? that defines the replication.

Every Node can serve the Objects without comunication with the Leader.

On the first implementation, the comunication between Nodes will be over HTTPS? and latter on via RPC

The Followers will not have any direct request to the Leader, they wait until the next heartbeat to inform the Leader of anithing that needs to be informed.

## Configuration

Te basic configuration is a .gogilefs.(json|yaml|xml) file located by default: __BLANK__ which can have this keys:

* storage: Array of locations in which Gogilefs will sotre the Objects
* name: Name of the cluster
* node_name: Canonical name for the node (readable logs)
* nodes: Array with a list of some of the nodes of the cluster
* classes/types: Map with the 'keys' beeing the names of the class and the values the reaplication.
* much more ...

<a name="objects_stored"></a>
## Objects Stored

Object can be anithing, from images to videos to anithing. The way we store them is making a SHAXXX and with the SHA key of length 40 we create subfolders for every X numbers (40/4=10 subfolders)

## Term

This idea is from Raft, the term identifis the "term" the current. Its a number incremented on each election, with this simple rule, we can identify old Nodes that try to be Leaders and are old.

<a name="node_role"></a>
## Node (follower) Role

A simple Node by itself can store Objects, Serve Objects to the client and Obey orders (replicates .. etc).

Each Node has an internal KV where it saves the Objects and the replica of the other servers.

Each Node has another internal DB/KV/StateMachine to store the current jobs, replicating, candidate, current_term etc.

Each Node has a LRU Cache to store the not know Objects.

### Store

When a Object needs to be stored:

* First is stores the object in a `tmp/` location (in case of crashing the server)
* Then it's copied to the location and removed from the `tmp/`
* Finally stores the SHA key to the KV store

If the Object needs to be replicated then the Node, in the next heartbeat will comunicate te pending replications to the Leader.

The response to the client can will be after the Object is saved on the Node or, if configured on the request/client, after the `w` (like MongoDB journals|j and write|w)

### Serve

If the Object needed is in the Node, the it serves the Object.

If the Object is not in his KV then it asks a "near" Node for the Object (and consecutivelty, if the one asked does not know the Object it asks another node, passing the nodes that do not know it so they do not repeat) one some node have the Object it response that it has the Object, all the "bypass" nodes store to the cache the value of the Object and the Node how have it. When the first Node recieves the Node how has the Object, then Proxyes the Object from the Node to the Client.

__NOTE__: If there are a lot of Nodes, should we stop the "ask near node for the Object" policy? When we consulted the majori and node of them has the Object?

### Status

The node itself can be in status:

* Leader
* Follower
* Candidate
* Draining ???

This status is saved (with other metadata) in disk, because if this node is restarted, then it needs to know what it was doing

#### Draning

The Draining status means that the server is going to be shootdown, so all the Objects must be replicated, and all the Nodes must be informed to remove the Draining Node from the list. If the object does not have any Object with replica, it can be shootdown saftly, because the other Nodes, once they remove the node from the KV of the Object, they will enter in replication.

### Comunication

Each heartbeat from the Leader, the followers may answer with:

* IO status
* Status
* Pending Replications
* The answer to the request of the heartbeat (if it brings information)

<a name="node_replication"></a>
### Replication

When a Node recives the order to replicate FROM A to B, the Node B will request the Object to the Node A (with some identifier of the request to prevent invalid replications). When the replication is compleated, the reciever (Node B) notifies the other nodes that the Object has been replicated successfully so they store that the Node B now also have a copy of the Object.


<a name="leader_role"></a>
## Leader Role

A cluster MUST have ONE and only ONE Leader.

The main job of the Leader is to track the `stats` of the Followers to start replicating, reorganizing the storage (if one Node is at XX% of storage then the Leaeder decides to move some of the Objects to another Node less full)

It also comunicates if one Node goes down to the other Nodes, in case some Objects needs to be replicated.

<a name="leader_replication"></a>
### Replication

When a Follower communicates to the Leader that it has an Object that need to be replicated, the master decides (with an algorith based on all the nodes `stats`) which Node will thake it, then it comunicates to both of the nodes that the Node A need to replicate to Node B, and after that it leaves the control to the Nodes itself.

If the replication if from more than 2 Nodes then all the Nodes must save the location of the Node that also have the Object when the Node B finishes the replication.

__NOTE__: If we have a global KV then the maste may know that the Object is alredy in one Node, and just communicate that the Object is there.
__NOTE__: If no Global KV, then the Master must ask the Followers if one of them have the Object. This cenario is if 2 Objects are uploaded at the same time, so 2 nodes enter in replication state.
__NOTE__: Maybe the good solution is the global KV store in case of 2 Objects uploaded at the same time.

<a name="leader_election"></a>
### Leader Electino

For the Leader election, as each Node is independent from the others (more or less) the approch thah Raft follows, without taking in conisderation the Raft Log, will fit.

Which is the following:


> Raft uses a heartbeat mechanism to trigger leader election. When servers start up, they begin as followers. A server remains in follower state as long as it receives valid RPCs from a leader or candidate. Leaders send periodic heartbeats (AppendEntries RPCs that carry no log entries) to all followers in order to maintain their authority. If a follower receives no communication over a period of time called the election timeout, then it assumes there is no vi- able leader and begins an election to choose a new leader.
>
> To begin an election, a follower increments its current term and transitions to candidate state. It then votes for itself and issues RequestVote RPCs in parallel to each of the other servers in the cluster. A candidate continues in this state until one of three things happens: (a) it wins the election, (b) another server establishes itself as leader, or (c) a period of time goes by with no winner. These out- comes are discussed separately in the paragraphs below.
>
> A candidate wins an election if it receives votes from a majority of the servers in the full cluster for the same term. Each server will vote for at most one candidate in a given term, on a first-come-first-served basis (note: Sec- tion 5.4 adds an additional restriction on votes). The ma- jority rule ensures that at most one candidate can win the election for a particular term (the Election Safety Prop- erty in Figure 3). Once a candidate wins an election, it becomes leader. It then sends heartbeat messages to all of the other servers to establish its authority and prevent new elections.
>
> While waiting for votes, a candidate may receive an AppendEntries RPC from another server claiming to be leader. If the leader’s term (included in its RPC) is at least as large as the candidate’s current term, then the candidate recognizes the leader as legitimate and returns to follower state. If the term in the RPC is smaller than the candidate’s current term, then the candidate rejects the RPC and con- tinues in candidate state.
>
> The third possible outcome is that a candidate neither wins nor loses the election: if many followers become candidates at the same time, votes could be split so that no candidate obtains a majority. When this happens, each candidate will time out and start a new election by incre- menting its term and initiating another round of Request- Vote RPCs. However, without extra measures split votes could repeat indefinitely.
>
> Raft uses randomized election timeouts to ensure that split votes are rare and that they are resolved quickly. To prevent split votes in the first place, election timeouts are chosen randomly from a fixed interval (e.g., 150–300ms). This spreads out the servers so that in most cases only a single server will time out; it wins the election and sends heartbeats before any other servers time out. The same mechanism is used to handle split votes. Each candidate restarts its randomized election timeout at the start of an election, and it waits for that timeout to elapse before starting the next election; this reduces the likelihood of another split vote in the new election.
>
> One of our requirements for Raft is that safety must not depend on timing: the system must not produce incor- rect results just because some event happens more quickly or slowly than expected. However, availability (the ability of the system to respond to clients in a timely manner) must inevitably depend on timing. For example, if mes- sage exchanges take longer than the typical time between server crashes, candidates will not stay up long enough to win an election; without a steady leader, Raft cannot make progress.
>
> Leader election is the aspect of Raft where timing is most critical. Raft will be able to elect and maintain a steady leader as long as the system satisfies the follow- ing timing requirement:
>
> broadcastTime ≪ electionTimeout ≪ MTBF
>
> In this inequality broadcastTime is the average time it takes a server to send RPCs in parallel to every server in the cluster and receive their responses; electionTime- out is the election timeout described in Section 5.2; and MTBF is the average time between failures for a single server. The broadcast time should be an order of mag- nitude less than the election timeout so that leaders can reliably send the heartbeat messages required to keep fol- lowers from starting elections; given the randomized ap- proach used for election timeouts, this inequality also makes split votes unlikely. The election timeout should be a few orders of magnitude less than MTBF so that the sys- tem makes steady progress. When the leader crashes, the system will be unavailable for roughly the election time- out; we would like this to represent only a small fraction of overall time.
>
> The broadcast time and MTBF are properties of the un- derlying system, while the election timeout is something we must choose. Raft’s RPCs typically require the recip- ient to persist information to stable storage, so the broad- cast time may range from 0.5ms to 20ms, depending on storage technology. As a result, the election timeout is likely to be somewhere between 10ms and 500ms.
>
> Tipical server MTBFs are several months or more, which easily satisfies the timing requirement.

Which in resume is the following:

* The Leader send heartbeat each 0.5-20ms
* The Node has a random tiemout (time without recieving a heartbeat) of 10-500ms
* If a Node timeouts, then it enters in candidate mode and starts election: voting for itself and asking the others to vote for him.
* It also increments the term (an incremental value wich refers to the number of the system total elections) and send it with the vote.
* Each Node votes only fore one candidate for each term.
* To win the election the Node must recieve the majority of the votes (3 of 5 i a 5 Node cluster)
* While in Candidate state a Node recieves a heartbeat of a Node, claiming to be Leader, with a term =< of his term then the Candidate Node returns to follower state and recognizes the leader.
* If no one wins the election, meaning that more that one node has entered in Candidate state, the the nodes will tiemout and restart an election in a random (for each Node) time and incrementing the current Term.
* Restart everithing again :)

<a name="new_node"></a>
### New Node

When a new Node is introduced to the cluster, it enters as a Follower and the master comunicates to all of the other Nodes that a new Node is on the Cluster.

Then the Leader starts replicating Objects to the new Node to balance all the cluster.

__NODE__: Some configuration for the maxim of input replcias?

# TODO

* Master replication is a RPC to itself?

http://stackoverflow.com/questions/23272663/transfer-a-big-file-in-golang
